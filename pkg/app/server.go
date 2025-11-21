package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/sync/errgroup"
)

// generateSelfSignedCert generates a self-signed certificate and key, saves them to disk,
// and returns the paths to the certificate and key files.
func generateSelfSignedCert(hostname, certDir string) (certPath, keyPath string, err error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("generate private key: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"OpenAudio Self-Signed Certificate"},
			CommonName:   hostname,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{hostname, "localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Create certificate directory if it doesn't exist
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return "", "", fmt.Errorf("create cert directory: %w", err)
	}

	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

	// Write certificate file
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return "", "", fmt.Errorf("write cert file: %w", err)
	}

	// Write key file
	if err := os.WriteFile(keyPath, privateKeyPEM, 0600); err != nil {
		return "", "", fmt.Errorf("write key file: %w", err)
	}

	return certPath, keyPath, nil
}

// sets up the echo server based on the config
func (app *App) runServer(ctx context.Context) error {
	config := app.config.OpenAudio.Server

	eg, ctx := errgroup.WithContext(ctx)

	// Start HTTP REST server (cleartext)
	if config.HTTP != nil && config.HTTP.Enabled {
		eg.Go(func() error {
			return app.startHTTPServer(ctx, config)
		})
	}

	// Start HTTPS REST server (TLS)
	if config.HTTPS != nil && config.HTTPS.Enabled {
		eg.Go(func() error {
			return app.startHTTPSServer(ctx, config)
		})
	}

	// Start gRPC h2c server (cleartext)
	if config.GRPC != nil && config.GRPC.Enabled {
		eg.Go(func() error {
			return app.startGRPCServer(ctx, config)
		})
	}

	// Start gRPC TLS server (encrypted)
	if config.GRPCS != nil && config.GRPCS.Enabled {
		eg.Go(func() error {
			return app.startGRPCSServer(ctx, config)
		})
	}

	// Start Unix socket server
	if config.Socket != nil && config.Socket.Enabled {
		eg.Go(func() error {
			return app.startSocketServer(ctx, config)
		})
	}

	if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (app *App) startHTTPServer(ctx context.Context, config *config.ServerConfig) error {
	addr := ":" + config.HTTP.Port
	app.logger.Info("Starting HTTP server", zap.String("addr", addr))

	// If HTTPS is also enabled, add redirect middleware
	if config.HTTPS != nil && config.HTTPS.Enabled {
		app.httpServer.Pre(middleware.HTTPSRedirect())
		app.logger.Info("HTTP server will redirect to HTTPS")
	}

	if err := app.httpServer.Start(addr); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("start HTTP server: %w", err)
	}
	return nil
}

func (app *App) startHTTPSServer(ctx context.Context, config *config.ServerConfig) error {
	addr := ":" + config.HTTPS.Port

	if config.HTTPS.SelfSigned {
		// Self-signed certificate mode
		app.logger.Info("Starting HTTPS server with self-signed certificate", zap.String("addr", addr))

		certPath, keyPath, err := generateSelfSignedCert(config.Hostname, config.HTTPS.CertDir)
		if err != nil {
			return fmt.Errorf("generate self-signed cert: %w", err)
		}

		tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return fmt.Errorf("load X509 key pair: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			NextProtos:   []string{"h2", "http/1.1"},
		}

		server := &http.Server{
			Addr:      addr,
			Handler:   app.httpServer,
			TLSConfig: tlsConfig,
		}

		// Configure HTTP/2
		if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
			return fmt.Errorf("configure http2: %w", err)
		}

		if err := server.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("start HTTPS server: %w", err)
		}
		return nil
	}

	// Autocert mode
	app.logger.Info("Starting HTTPS server with autocert", zap.String("addr", addr))

	// Build whitelist for autocert
	whitelist := []string{config.Hostname, "localhost"}
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				whitelist = append(whitelist, ip4.String())
			}
		}
	}

	app.httpServer.AutoTLSManager.HostPolicy = autocert.HostWhitelist(whitelist...)
	app.httpServer.AutoTLSManager.Cache = autocert.DirCache(config.HTTPS.CacheDir)

	if err := app.httpServer.StartAutoTLS(addr); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("start HTTPS server with autocert: %w", err)
	}
	return nil
}

func (app *App) startGRPCServer(ctx context.Context, config *config.ServerConfig) error {
	addr := ":" + config.GRPC.Port
	app.logger.Info("Starting gRPC server with h2c", zap.String("addr", addr))

	h2cHandler := h2c.NewHandler(app.grpcServer, &http2.Server{})
	server := &http.Server{
		Addr:    addr,
		Handler: h2cHandler,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("start gRPC h2c server: %w", err)
	}
	return nil
}

func (app *App) startGRPCSServer(ctx context.Context, config *config.ServerConfig) error {
	addr := ":" + config.GRPCS.Port
	app.logger.Info("Starting gRPC server with TLS", zap.String("addr", addr))

	var certPath, keyPath string
	var err error

	if config.GRPCS.SelfSigned {
		// Generate self-signed certificate
		certPath, keyPath, err = generateSelfSignedCert(config.Hostname, config.GRPCS.CertDir)
		if err != nil {
			return fmt.Errorf("generate self-signed cert for gRPCS: %w", err)
		}
	} else {
		// Use existing certificates from autocert cache
		certPath = filepath.Join(config.GRPCS.CertDir, "cert.pem")
		keyPath = filepath.Join(config.GRPCS.CertDir, "key.pem")
	}

	tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("load X509 key pair for gRPCS: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"h2"},
	}

	server := &http.Server{
		Addr:      addr,
		Handler:   app.grpcsServer,
		TLSConfig: tlsConfig,
	}

	// Configure HTTP/2
	if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
		return fmt.Errorf("configure http2 for gRPCS: %w", err)
	}

	if err := server.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("start gRPC TLS server: %w", err)
	}
	return nil
}

func (app *App) startSocketServer(ctx context.Context, config *config.ServerConfig) error {
	socketPath := config.Socket.Path
	app.logger.Info("Starting Unix socket server", zap.String("path", socketPath))

	// Remove existing socket file if it exists
	if err := os.RemoveAll(socketPath); err != nil {
		return fmt.Errorf("remove existing socket file: %w", err)
	}

	if err := app.socketServer.Start("unix:" + socketPath); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("start Unix socket server: %w", err)
	}
	return nil
}
