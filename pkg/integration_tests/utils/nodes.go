package utils

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
)

var (
	DiscoveryOneRPC = getEnvWithDefault("discoveryOneRPC", "node1.oap.devnet")
	ContentOneRPC   = getEnvWithDefault("contentOneRPC", "node2.oap.devnet")
	ContentTwoRPC   = getEnvWithDefault("contentTwoRPC", "node3.oap.devnet")
	ContentThreeRPC = getEnvWithDefault("contentThreeRPC", "node4.oap.devnet")

	DiscoveryOne *sdk.OpenAudioSDK
	ContentOne   *sdk.OpenAudioSDK
	ContentTwo   *sdk.OpenAudioSDK
	ContentThree *sdk.OpenAudioSDK
)

// NewTestHTTPClient creates an HTTP client configured for local devnet testing.
// It skips TLS verification to work with self-signed certificates while maintaining HTTPS protocol.
func NewTestHTTPClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}
}

// NewTestSDK creates a new SDK instance with the test HTTP client.
// Use this when you need to create SDK instances in tests instead of using the pre-configured ones.
func NewTestSDK(nodeURL string) *sdk.OpenAudioSDK {
	return sdk.NewOpenAudioSDKWithClient(nodeURL, NewTestHTTPClient())
}

func init() {
	// Use custom HTTP client that skips TLS verification for self-signed certs in devnet
	// This maintains HTTPS protocol (as expected by the server) but allows local testing
	httpClient := NewTestHTTPClient()
	DiscoveryOne = sdk.NewOpenAudioSDKWithClient(DiscoveryOneRPC, httpClient)
	ContentOne = sdk.NewOpenAudioSDKWithClient(ContentOneRPC, httpClient)
	ContentTwo = sdk.NewOpenAudioSDKWithClient(ContentTwoRPC, httpClient)
	ContentThree = sdk.NewOpenAudioSDKWithClient(ContentThreeRPC, httpClient)
}

func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func EnsureProtocol(endpoint string) string {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return "http://" + endpoint
	}
	return endpoint
}

type devnetNode struct {
	name string
	rpc  string
	sdk  *sdk.OpenAudioSDK
}

type readinessProbe struct {
	name             string
	rpc              string
	statusReady      bool
	height           int64
	peerCount        int
	statusErr        string
	healthStatusCode int
	storageHealthy   bool
	walletRegistered bool
	healthErr        string
}

func (p readinessProbe) ready(minHeight int64) bool {
	return p.statusErr == "" &&
		p.statusReady &&
		p.height >= minHeight &&
		p.healthErr == "" &&
		p.healthStatusCode == http.StatusOK &&
		p.storageHealthy &&
		p.walletRegistered
}

func (p readinessProbe) String() string {
	parts := []string{
		fmt.Sprintf("%s(%s)", p.name, p.rpc),
		fmt.Sprintf("ready=%t", p.statusReady),
		fmt.Sprintf("height=%d", p.height),
		fmt.Sprintf("peers=%d", p.peerCount),
		fmt.Sprintf("storage_healthy=%t", p.storageHealthy),
		fmt.Sprintf("wallet_registered=%t", p.walletRegistered),
	}
	if p.healthStatusCode != 0 {
		parts = append(parts, fmt.Sprintf("health_status=%d", p.healthStatusCode))
	}
	if p.statusErr != "" {
		parts = append(parts, "status_err="+p.statusErr)
	}
	if p.healthErr != "" {
		parts = append(parts, "health_err="+p.healthErr)
	}
	return strings.Join(parts, " ")
}

func healthCheckURL(addr string) string {
	baseURL := addr
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		baseURL = "https://" + baseURL
	} else if strings.HasPrefix(baseURL, "http://") {
		baseURL = strings.Replace(baseURL, "http://", "https://", 1)
	}
	return strings.TrimRight(baseURL, "/") + "/health-check"
}

func formatReadinessProbes(probes []readinessProbe) string {
	if len(probes) == 0 {
		return "no readiness probes captured"
	}
	parts := make([]string, 0, len(probes))
	for _, probe := range probes {
		parts = append(parts, probe.String())
	}
	return strings.Join(parts, "; ")
}

func probeDevnetReadiness(ctx context.Context, nodes []devnetNode, pollClient *http.Client) []readinessProbe {
	probes := make([]readinessProbe, 0, len(nodes))
	for _, node := range nodes {
		probe := readinessProbe{name: node.name, rpc: node.rpc}

		reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
		status, err := node.sdk.Core.GetStatus(reqCtx, connect.NewRequest(&corev1.GetStatusRequest{}))
		reqCancel()
		if err != nil {
			probe.statusErr = err.Error()
		} else if status.Msg == nil {
			probe.statusErr = "empty status response"
		} else {
			probe.statusReady = status.Msg.GetReady()
			probe.height = status.Msg.GetChainInfo().GetCurrentHeight()
			probe.peerCount = len(status.Msg.GetPeers().GetPeers())
		}

		reqCtx, reqCancel = context.WithTimeout(ctx, 5*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, "GET", healthCheckURL(node.rpc), nil)
		if err != nil {
			probe.healthErr = err.Error()
			reqCancel()
			probes = append(probes, probe)
			continue
		}
		resp, err := pollClient.Do(req)
		reqCancel()
		if err != nil {
			probe.healthErr = err.Error()
			probes = append(probes, probe)
			continue
		}

		probe.healthStatusCode = resp.StatusCode
		var healthResponse struct {
			Storage struct {
				Healthy            bool `json:"healthy"`
				WalletIsRegistered bool `json:"wallet_is_registered"`
			} `json:"storage"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&healthResponse); err != nil {
			probe.healthErr = err.Error()
		} else {
			probe.storageHealthy = healthResponse.Storage.Healthy
			probe.walletRegistered = healthResponse.Storage.WalletIsRegistered
		}
		resp.Body.Close()
		probes = append(probes, probe)
	}
	return probes
}

func WaitForDevnetHealthy(timeout ...time.Duration) error {
	return WaitForDevnetReady(3, timeout...)
}

func WaitForDevnetReady(minHeight int64, timeout ...time.Duration) error {
	timeoutDuration := 300 * time.Second
	if len(timeout) > 0 {
		timeoutDuration = timeout[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	nodes := []devnetNode{
		{name: "discovery-one", rpc: DiscoveryOneRPC, sdk: DiscoveryOne},
		{name: "content-one", rpc: ContentOneRPC, sdk: ContentOne},
		{name: "content-two", rpc: ContentTwoRPC, sdk: ContentTwo},
		{name: "content-three", rpc: ContentThreeRPC, sdk: ContentThree},
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Use a short per-request timeout so a single slow/hung node cannot
	// exhaust the overall readiness budget on one iteration.
	pollClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}

	checkReady := func() ([]readinessProbe, bool) {
		probes := probeDevnetReadiness(ctx, nodes, pollClient)
		for _, probe := range probes {
			if !probe.ready(minHeight) {
				return probes, false
			}
		}
		return probes, true
	}

	lastProbes, ready := checkReady()
	if ready {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			if len(lastProbes) == 0 {
				return errors.New("timed out waiting for devnet to be ready")
			}
			return fmt.Errorf(
				"timed out waiting for devnet to be ready after %s (min_height=%d): %s",
				timeoutDuration,
				minHeight,
				formatReadinessProbes(lastProbes),
			)
		case <-ticker.C:
			lastProbes, ready = checkReady()
			if ready {
				return nil
			}
		}
	}
}
