package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetHome(t *testing.T) {
	// given
	cfg := DefaultConfig()
	oldHome := cfg.OpenAudio.Home
	newHome := "/tmp/openaudio-test-home"

	// sanity check baseline
	require.Equal(t, DefaultHomeDir, oldHome)
	require.True(t, strings.HasPrefix(cfg.OpenAudio.Blob.BlobStoreDSN, "file://"))

	// when
	cfg.SetHome(newHome)

	// then
	require.Equal(t, newHome, cfg.CometBFT.RootDir)
	require.Equal(t, newHome, cfg.OpenAudio.Home)

	// subconfigs should all have updated homes
	require.Equal(t, newHome, cfg.OpenAudio.Version.Home)
	require.Equal(t, newHome, cfg.OpenAudio.Eth.Home)
	require.Equal(t, newHome, cfg.OpenAudio.DB.Home)
	require.Equal(t, newHome, cfg.OpenAudio.Blob.Home)
	require.Equal(t, newHome, cfg.OpenAudio.Operator.Home)
	require.Equal(t, newHome, cfg.OpenAudio.Server.Home)
	require.Equal(t, newHome, cfg.OpenAudio.Server.TLS.Home)
	require.Equal(t, newHome, cfg.OpenAudio.Server.Console.Home)
	require.Equal(t, newHome, cfg.OpenAudio.Server.Socket.Home)

	// derived paths recomputed correctly
	expectedBlobDir := "file://" + filepath.Join(newHome, DefaultDataDir, DefaultBlobsDir)
	require.Equal(t, expectedBlobDir, cfg.OpenAudio.Blob.BlobStoreDSN)

	expectedSocketPath := filepath.Join(newHome, DefaultSocketFileName)
	require.Equal(t, expectedSocketPath, cfg.OpenAudio.Server.Socket.Path)

	expectedCertDir := filepath.Join(newHome, DefaultConfigDir, DefaultCertsDir)
	require.Equal(t, expectedCertDir, cfg.OpenAudio.Server.TLS.CertDir)

	expectedCacheDir := filepath.Join(newHome, DefaultConfigDir, DefaultCacheDir)
	require.Equal(t, expectedCacheDir, cfg.OpenAudio.Server.TLS.CacheDir)

	// ensure defaults not affected elsewhere
	require.Equal(t, DefaultHTTPPort, cfg.OpenAudio.Server.Port)
	require.Equal(t, DefaultRPCURL, cfg.OpenAudio.Eth.RpcURL)
	require.Equal(t, DefaultPostgresDSN, cfg.OpenAudio.DB.PostgresDSN)
}
