package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/mediorum/ethcontracts"
	"github.com/OpenAudio/go-openaudio/pkg/mediorum/server/signature"
	"github.com/OpenAudio/go-openaudio/pkg/registrar"
	"github.com/gowebpki/jcs"
	"github.com/labstack/echo/v4"
)

type HealthCheckResponse struct {
	Data      HealthCheckResponseData `json:"data"`
	Signer    string                  `json:"signer"`
	Signature string                  `json:"signature"`
	Timestamp time.Time               `json:"timestamp"`
}

type HealthCheckResponseData struct {
	Healthy                 bool                       `json:"healthy"`
	Version                 string                     `json:"version"`
	Service                 string                     `json:"service"` // used by registerWithDelegate()
	IsSeeding               bool                       `json:"isSeeding"`
	BuiltAt                 string                     `json:"builtAt"`
	StartedAt               time.Time                  `json:"startedAt"`
	SPID                    int                        `json:"spID"`
	SPOwnerWallet           string                     `json:"spOwnerWallet"`
	Git                     string                     `json:"git"`
	MediorumPathUsed        uint64                     `json:"mediorumPathUsed"`   // bytes
	MediorumPathSize        uint64                     `json:"mediorumPathSize"`   // bytes
	StorageExpectation      uint64                     `json:"storageExpectation"` // bytes
	DatabaseSize            uint64                     `json:"databaseSize"`       // bytes
	DbSizeErr               string                     `json:"dbSizeErr"`
	LastSuccessfulRepair    RepairTracker              `json:"lastSuccessfulRepair"`
	LastSuccessfulCleanup   RepairTracker              `json:"lastSuccessfulCleanup"`
	UploadsCount            int64                      `json:"uploadsCount"`
	UploadsCountErr         string                     `json:"uploadsCountErr"`
	TrustedNotifier         *ethcontracts.NotifierInfo `json:"trustedNotifier"`
	Env                     string                     `json:"env"`
	Self                    registrar.Peer             `json:"self"`
	WalletIsRegistered      bool                       `json:"wallet_is_registered"`
	Signers                 []registrar.Peer           `json:"signers"`
	ReplicationFactor       int                        `json:"replicationFactor"`
	Dir                     string                     `json:"dir"`
	BlobStorePrefix         string                     `json:"blobStorePrefix"`
	MoveFromBlobStorePrefix string                     `json:"moveFromBlobStorePrefix"`
	ListenPort              string                     `json:"listenPort"`
	PeerHealths             map[string]*PeerHealth     `json:"peerHealths"`
	UnreachablePeers        []string                   `json:"unreachablePeers"`
	FailsPeerReachability   bool                       `json:"failsPeerReachability"`
	StoreAll                bool                       `json:"storeAll"`
	DiskHasSpace            bool                       `json:"diskHasSpace"`
	TranscodeQueueLength    int                        `json:"transcodeQueueLength"`
	TranscodeStats          *TranscodeStats            `json:"transcodeStats"`
}

func (ss *MediorumServer) serveHealthCheck(c echo.Context) error {
	healthy := ss.databaseSize > 0 && ss.dbSizeErr == "" && ss.uploadsCountErr == ""

	allowUnregistered, _ := strconv.ParseBool(c.QueryParam("allow_unregistered"))
	if !allowUnregistered && !ss.Config.WalletIsRegistered {
		healthy = false
	}

	blobStorePrefix, _, foundBlobStore := strings.Cut(ss.Config.OpenAudio.Blob.BlobStoreDSN, "://")
	if !foundBlobStore {
		blobStorePrefix = ""
	}
	blobStoreMoveFromPrefix, _, foundBlobStoreMoveFrom := strings.Cut(ss.Config.OpenAudio.Blob.MoveFromBlobStoreDSN, "://")
	if !foundBlobStoreMoveFrom {
		blobStoreMoveFromPrefix = ""
	}

	var err error
	// since we're using peerHealth
	ss.peerHealthsMutex.RLock()
	defer ss.peerHealthsMutex.RUnlock()

	data := HealthCheckResponseData{
		Healthy:                 healthy,
		Version:                 ss.Config.OpenAudio.Version.Tag,
		Service:                 "validator", // TODO: validator, archive, rpc, light, seed
		IsSeeding:               ss.isSeeding,
		BuiltAt:                 vcsBuildTime,
		StartedAt:               ss.StartedAt,
		SPID:                    ss.Config.SPID,          // TODO: get from ethService
		SPOwnerWallet:           ss.Config.SPOwnerWallet, // TODO: get from ethService
		Git:                     ss.Config.OpenAudio.Version.GitSHA,
		MediorumPathUsed:        ss.mediorumPathUsed,
		MediorumPathSize:        ss.mediorumPathSize,
		StorageExpectation:      ss.storageExpectation,
		DatabaseSize:            ss.databaseSize,
		DbSizeErr:               ss.dbSizeErr,
		LastSuccessfulRepair:    ss.lastSuccessfulRepair,
		LastSuccessfulCleanup:   ss.lastSuccessfulCleanup,
		UploadsCount:            ss.uploadsCount,
		UploadsCountErr:         ss.uploadsCountErr,
		TrustedNotifier:         ss.trustedNotifier,
		Dir:                     ss.Config.OpenAudio.Storage.Dir,
		BlobStorePrefix:         blobStorePrefix,
		MoveFromBlobStorePrefix: blobStoreMoveFromPrefix,
		ListenPort:              ss.Config.OpenAudio.Server.HTTPSPort,
		ReplicationFactor:       int(ss.Config.GenesisData.Storage.ReplicationFactor),
		Env:                     ss.Config.GenesisDoc.ChainID,
		Self:                    ss.Config.Self,
		WalletIsRegistered:      ss.Config.WalletIsRegistered,
		PeerHealths:             ss.peerHealths,
		UnreachablePeers:        ss.unreachablePeers,
		FailsPeerReachability:   ss.failsPeerReachability,
		Signers:                 ss.Config.Signers,
		StoreAll:                ss.Config.OpenAudio.Storage.StoreAll,
		DiskHasSpace:            ss.diskHasSpace(),
		TranscodeQueueLength:    len(ss.transcodeWork),
		TranscodeStats:          ss.getTranscodeStats(),
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return c.JSON(500, map[string]string{"error": "Failed to marshal health check data: " + err.Error()})
	}
	dataBytesSorted, err := jcs.Transform(dataBytes)
	if err != nil {
		return c.JSON(500, map[string]string{"error": "Failed to sort health check data: " + err.Error()})
	}
	signatureHex := "private key not set (should only happen on local dev)!"
	if ss.Config.PrivKey != nil {
		signature, err := signature.SignBytes(dataBytesSorted, ss.Config.PrivKey)
		if err != nil {
			return c.JSON(500, map[string]string{"error": "Failed to sign health check response: " + err.Error()})
		}
		signatureHex = fmt.Sprintf("0x%s", hex.EncodeToString(signature))
	}

	status := 200
	if !healthy {
		if !allowUnregistered && !ss.Config.WalletIsRegistered {
			status = 506
		} else {
			status = 503
		}
	}

	return c.JSON(status, HealthCheckResponse{
		Data:      data,
		Signer:    ss.Config.OpenAudio.Operator.EthAddress,
		Signature: signatureHex,
		Timestamp: time.Now(),
	})
}

func isDbLocalhost(postgresDSN string) bool {
	switch postgresDSN {
	case "postgres://postgres:postgres@db:5432/audius_creator_node", "postgresql://postgres:postgres@db:5432/audius_creator_node", "postgres://postgres:postgres@db:5432/openaudio", "postgresql://postgres:postgres@db:5432/openaudio", "localhost":
		return true
	default:
		return false
	}
}

func (ss *MediorumServer) requireHealthy(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		allowUnhealthy, _ := strconv.ParseBool(c.QueryParam("allow_unhealthy"))
		if allowUnhealthy {
			return next(c)
		}

		if !ss.Config.WalletIsRegistered {
			return c.JSON(506, "wallet not registered")
		}
		dbHealthy := ss.databaseSize > 0 && ss.dbSizeErr == "" && ss.uploadsCountErr == ""
		if !dbHealthy {
			return c.JSON(503, "database not healthy")
		}

		return next(c)
	}
}
