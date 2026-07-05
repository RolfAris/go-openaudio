package utils

import (
	"strings"
	"testing"
)

func TestHealthCheckURL(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "host only",
			addr: "node1.oap.devnet",
			want: "https://node1.oap.devnet/health-check",
		},
		{
			name: "http is upgraded",
			addr: "http://node1.oap.devnet",
			want: "https://node1.oap.devnet/health-check",
		},
		{
			name: "https is preserved",
			addr: "https://node1.oap.devnet",
			want: "https://node1.oap.devnet/health-check",
		},
		{
			name: "trailing slash",
			addr: "https://node1.oap.devnet/",
			want: "https://node1.oap.devnet/health-check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthCheckURL(tt.addr); got != tt.want {
				t.Fatalf("healthCheckURL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestReadinessProbeReady(t *testing.T) {
	readyProbe := readinessProbe{
		statusReady:      true,
		height:           3,
		healthStatusCode: 200,
		storageHealthy:   true,
		walletRegistered: true,
	}
	if !readyProbe.ready(3) {
		t.Fatal("expected fully ready probe to pass")
	}

	tests := []struct {
		name  string
		probe readinessProbe
	}{
		{
			name:  "status not ready",
			probe: readinessProbe{statusReady: false, height: 3, healthStatusCode: 200, storageHealthy: true, walletRegistered: true},
		},
		{
			name:  "height below minimum",
			probe: readinessProbe{statusReady: true, height: 2, healthStatusCode: 200, storageHealthy: true, walletRegistered: true},
		},
		{
			name:  "storage unhealthy",
			probe: readinessProbe{statusReady: true, height: 3, healthStatusCode: 200, storageHealthy: false, walletRegistered: true},
		},
		{
			name:  "wallet not registered",
			probe: readinessProbe{statusReady: true, height: 3, healthStatusCode: 200, storageHealthy: true, walletRegistered: false},
		},
		{
			name:  "health error",
			probe: readinessProbe{statusReady: true, height: 3, healthStatusCode: 200, storageHealthy: true, walletRegistered: true, healthErr: "dial failed"},
		},
		{
			name:  "status error",
			probe: readinessProbe{statusReady: true, height: 3, healthStatusCode: 200, storageHealthy: true, walletRegistered: true, statusErr: "rpc failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.probe.ready(3) {
				t.Fatalf("expected probe to fail readiness: %+v", tt.probe)
			}
		})
	}
}

func TestFormatReadinessProbesIncludesDiagnostics(t *testing.T) {
	got := formatReadinessProbes([]readinessProbe{
		{
			name:             "content-one",
			rpc:              "node2.oap.devnet",
			statusReady:      true,
			height:           2,
			peerCount:        3,
			healthStatusCode: 200,
			storageHealthy:   false,
			walletRegistered: true,
			healthErr:        "storage cold",
		},
	})

	for _, want := range []string{
		"content-one(node2.oap.devnet)",
		"ready=true",
		"height=2",
		"peers=3",
		"storage_healthy=false",
		"wallet_registered=true",
		"health_status=200",
		"health_err=storage cold",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatReadinessProbes() = %q, missing %q", got, want)
		}
	}
}
