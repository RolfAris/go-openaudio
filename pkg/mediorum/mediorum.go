package mediorum

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"github.com/OpenAudio/go-openaudio/pkg/config"
	coreServer "github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/OpenAudio/go-openaudio/pkg/lifecycle"
	"github.com/OpenAudio/go-openaudio/pkg/mediorum/ethcontracts"
	"github.com/OpenAudio/go-openaudio/pkg/mediorum/server"
	"github.com/OpenAudio/go-openaudio/pkg/pos"
	"github.com/OpenAudio/go-openaudio/pkg/registrar"
	"go.uber.org/zap"
	"golang.org/x/exp/slog"
	"golang.org/x/sync/errgroup"
)

func Run(lc *lifecycle.Lifecycle, logger *zap.Logger, posChannel chan pos.PoSRequest, storageService *server.StorageService, core *coreServer.CoreService) error {
	env := os.Getenv("OPENAUDIO_ENV")
	logger.Info("starting mediorum", zap.String("OPENAUDIO_ENV", env))

	return runMediorum(lc, logger, env, posChannel, storageService, core)
}

func runMediorum(lc *lifecycle.Lifecycle, logger *zap.Logger, config *config.Config, posChannel chan pos.PoSRequest, storageService *server.StorageService, core *coreServer.CoreService) error {
	logger = logger.With(zap.String("service", "mediorum"))

	isProd := mediorumEnv == "prod"
	isStage := mediorumEnv == "stage"
	isDev := mediorumEnv == "dev"

	var g registrar.PeerProvider
	if isProd {
		g = registrar.NewMultiProd()
	}
	if isStage {
		g = registrar.NewMultiStaging()
	}
	if isDev {
		g = registrar.NewMultiDev()
	}

	var peers, signers []registrar.Peer
	var err error

	eg := new(errgroup.Group)
	eg.Go(func() error {
		peers, err = g.Peers()
		return err
	})
	eg.Go(func() error {
		signers, err = g.Signers()
		return err
	})
	if err := eg.Wait(); err != nil {
		panic(err)
	}
	logger.Info("fetched registered nodes", zap.Int("peers", len(peers)), zap.Int("signers", len(signers)))

	nodeEndpoint := os.Getenv("nodeEndpoint")
	if nodeEndpoint == "" {
		return errors.New("missing required env variable 'nodeEndpoint'")
	}
	privateKeyHex := os.Getenv("delegatePrivateKey")
	if privateKeyHex == "" {
		return errors.New("missing required env variable 'delegatePrivateKey'")
	}

	privateKey, err := ethcontracts.ParsePrivateKeyHex(privateKeyHex)
	if err != nil {
		return fmt.Errorf("invalid private key: %v", err)
	}

	// compute wallet address
	walletAddress := ethcontracts.ComputeAddressFromPrivateKey(privateKey)
	delegateOwnerWallet := os.Getenv("delegateOwnerWallet")
	if !strings.EqualFold(walletAddress, delegateOwnerWallet) {
		slog.Warn("incorrect delegateOwnerWallet env config", "incorrect", delegateOwnerWallet, "computed", walletAddress)
	}

	trustedNotifierID, err := strconv.Atoi(getenvWithDefault("trustedNotifierID", "1"))
	if err != nil {
		logger.Warn("failed to parse trustedNotifierID", zap.Error(err))
	}
	spID, err := ethcontracts.GetServiceProviderIdFromEndpoint(nodeEndpoint, walletAddress)
	if err != nil || spID == 0 {
		go func() {
			for range time.Tick(10 * time.Second) {
				logger.Warn("failed to recover spID - please register at https://dashboard.audius.org and restart the server", zap.Error(err))
			}
		}()
	}

	ss, err := server.New(lc, logger, config, g, posChannel, core)
	if err != nil {
		return fmt.Errorf("failed to create server: %v", err)
	}

	storageService.SetMediorum(ss)
	return ss.MustStart()
}

func getenvWithDefault(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
