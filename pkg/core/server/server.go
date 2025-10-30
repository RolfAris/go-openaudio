package server

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	"github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/OpenAudio/go-openaudio/pkg/core/db"
	"github.com/OpenAudio/go-openaudio/pkg/eth"
	"github.com/OpenAudio/go-openaudio/pkg/lifecycle"
	"github.com/OpenAudio/go-openaudio/pkg/pos"
	"github.com/OpenAudio/go-openaudio/pkg/pubsub"
	"github.com/OpenAudio/go-openaudio/pkg/rewards"
	"github.com/OpenAudio/go-openaudio/pkg/safemap"
	cconfig "github.com/cometbft/cometbft/config"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/rpc/client/local"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// subscribes by tx hash, pubsub completes once tx
// is committed
type TransactionHashPubsub = pubsub.Pubsub[struct{}]

type Server struct {
	lc             *lifecycle.Lifecycle
	config         *config.Config
	cometbftConfig *cconfig.Config
	logger         *zap.Logger
	self           corev1connect.CoreServiceClient
	eth            *eth.EthService

	pool               *pgxpool.Pool
	mediorumPoSChannel chan pos.PoSRequest

	db    *db.Queries
	node  *nm.Node
	rpc   *local.Local
	mempl *Mempool

	connectRPCPeers  *safemap.SafeMap[EthAddress, corev1connect.CoreServiceClient]
	cometRPCPeers    *safemap.SafeMap[EthAddress, *CometBFTRPC]
	cometListenAddrs *safemap.SafeMap[CometBFTAddress, CometBFTListener]
	peerStatus       *safemap.SafeMap[EthAddress, *v1.GetStatusResponse_PeerInfo_Peer]

	txPubsub       *TransactionHashPubsub
	blockNumPubsub *BlockNumPubsub

	cache     *Cache
	abciState *ABCIState

	rewards *rewards.RewardAttester

	awaitRpcReady chan struct{}
	awaitEthReady chan struct{}
}

func NewServer(lc *lifecycle.Lifecycle, config *config.Config, cconfig *cconfig.Config, logger *zap.Logger, pool *pgxpool.Pool, ethService *eth.EthService, posChannel chan pos.PoSRequest) *Server {
	// create mempool
	mempl := NewMempool(logger, config, db.New(pool), cconfig.Mempool.Size)

	// create pubsubs
	txPubsub := pubsub.NewPubsub[struct{}]()
	blockNumPubsub := pubsub.NewPubsub[int64]()

	z := logger.With(zap.String("service", "core"))
	z.Info("core server starting")

	coreLifecycle := lifecycle.NewFromLifecycle(lc, "core")

	s := &Server{
		lc:             coreLifecycle,
		config:         config,
		cometbftConfig: cconfig,
		logger:         z,
		eth:            ethService,

		pool:               pool,
		mediorumPoSChannel: posChannel,

		db:               db.New(pool),
		mempl:            mempl,
		connectRPCPeers:  safemap.New[EthAddress, corev1connect.CoreServiceClient](),
		cometRPCPeers:    safemap.New[EthAddress, *CometBFTRPC](),
		cometListenAddrs: safemap.New[CometBFTAddress, CometBFTListener](),
		peerStatus:       safemap.New[EthAddress, *v1.GetStatusResponse_PeerInfo_Peer](),
		txPubsub:         txPubsub,
		blockNumPubsub:   blockNumPubsub,
		cache:            NewCache(config),
		abciState:        NewABCIState(config.RetainHeight),

		rewards: rewards.NewRewardAttester(config.EthereumKey, config.Rewards),

		awaitRpcReady: make(chan struct{}),
		awaitEthReady: make(chan struct{}),
	}

	return s
}

func (s *Server) Init() error {
	logger := s.logger

	logger.Info("core server initializing")

	if err := db.RunMigrations(s.logger, s.config.PSQLConn, false); err != nil {
		return fmt.Errorf("running migrations: %v", err)
	}

	logger.Info("db migrations successful")

	s.CompactStateDB()
	s.CompactBlockstoreDB()
	logger.Info("finished compacting db")

	return nil
}

func (s *Server) Run() error {
	s.lc.AddManagedRoutine("abci", s.startABCI)
	s.lc.AddManagedRoutine("registry bridge", s.startRegistryBridge)
	s.lc.AddManagedRoutine("sync tasks", s.startSyncTasks)
	s.lc.AddManagedRoutine("cache", s.startCache)
	s.lc.AddManagedRoutine("data companion", s.startDataCompanion)
	s.lc.AddManagedRoutine("log sync", s.syncLogs)
	s.lc.AddManagedRoutine("state sync", s.startStateSync)
	s.lc.AddManagedRoutine("mempool cache", s.startMempoolCache)
	s.lc.AddManagedRoutine("peer manager", s.managePeers)
	s.lc.AddManagedRoutine("tx count cache", s.cacheTxCount)

	s.logger.Info("routines started")

	s.lc.Wait()
	return nil
}

func (s *Server) Start() error {
	s.lc.AddManagedRoutine("abci", s.startABCI)
	s.lc.AddManagedRoutine("registry bridge", s.startRegistryBridge)
	s.lc.AddManagedRoutine("sync tasks", s.startSyncTasks)
	s.lc.AddManagedRoutine("cache", s.startCache)
	s.lc.AddManagedRoutine("data companion", s.startDataCompanion)
	s.lc.AddManagedRoutine("log sync", s.syncLogs)
	s.lc.AddManagedRoutine("state sync", s.startStateSync)
	s.lc.AddManagedRoutine("mempool cache", s.startMempoolCache)
	s.lc.AddManagedRoutine("peer manager", s.managePeers)
	s.lc.AddManagedRoutine("tx count cache", s.cacheTxCount)

	s.logger.Info("routines started")

	s.lc.Wait()
	return fmt.Errorf("core stopped or shut down")
}

func (s *Server) setSelf(self corev1connect.CoreServiceClient) {
	s.self = self
}

func (s *Server) GetConfig() *config.Config {
	return s.config
}

func (s *Server) syncLogs(ctx context.Context) error {
	s.StartProcess(ProcessStateLogSync)

	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			s.RunningProcessWithMetadata(ProcessStateLogSync, "Syncing log buffers")
			s.logger.Sync()
			s.SleepingProcessWithMetadata(ProcessStateLogSync, "Waiting for next sync")
		case <-ctx.Done():
			s.CompleteProcess(ProcessStateLogSync)
			return ctx.Err()
		}
	}
}

func (s *Server) Shutdown() error {
	s.logger.Info("shutting down all services...")

	if err := s.lc.ShutdownWithTimeout(60 * time.Second); err != nil {
		return fmt.Errorf("failure shutting down core: %v", err)
	}

	return nil
}
