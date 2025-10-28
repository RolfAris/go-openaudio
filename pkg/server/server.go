package server

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
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.akshayshah.org/connectproto"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/encoding/protojson"

	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	ethv1connect "github.com/OpenAudio/go-openaudio/pkg/api/eth/v1/v1connect"
	storagev1connect "github.com/OpenAudio/go-openaudio/pkg/api/storage/v1/v1connect"
	systemv1connect "github.com/OpenAudio/go-openaudio/pkg/api/system/v1/v1connect"
)

var (
	marshalOpts   = protojson.MarshalOptions{EmitUnpopulated: true}
	unmarshalOpts = protojson.UnmarshalOptions{DiscardUnknown: true}

	// Compose them into the Connect handler option
	connectJSONOpt = connectproto.WithJSON(marshalOpts, unmarshalOpts)
)

type Server struct {
	ctx    context.Context
	config *config.Config
	logger *zap.Logger
	e      *echo.Echo

	core    *CoreServer
	storage *StorageServer
	system  *SystemServer
	eth     *EthServer
}

func NewServer(ctx context.Context, config *config.Config, logger *zap.Logger) *Server {
	return &Server{
		ctx:    ctx,
		config: config,
		logger: logger,
		e:      echo.New(),
	}
}

func (s *Server) Init() error {
	ecfg := s.config.OpenAudio.Server.Echo

	e := s.e
	core := s.core
	storage := s.storage
	system := s.system
	eth := s.eth

	e.HideBanner = true
	e.Logger = (*ZapEchoLogger)(s.logger)

	e.Use(middleware.CORS())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(ecfg.IPRateLimit))))
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout: ecfg.RequestTimeout,
	}))

	// ConnectRPC Routes
	rpcGroup := e.Group("")
	corePath, coreHandler := corev1connect.NewCoreServiceHandler(core, connectJSONOpt)
	rpcGroup.POST(corePath+"*", echo.WrapHandler(coreHandler))
	rpcGroup.GET(corePath+"*", echo.WrapHandler(coreHandler))

	storagePath, storageHandler := storagev1connect.NewStorageServiceHandler(storage, connectJSONOpt)
	rpcGroup.POST(storagePath+"*", echo.WrapHandler(storageHandler))
	rpcGroup.GET(storagePath+"*", echo.WrapHandler(storageHandler))

	systemPath, systemHandler := systemv1connect.NewSystemServiceHandler(system, connectJSONOpt)
	rpcGroup.POST(systemPath+"*", echo.WrapHandler(systemHandler))
	rpcGroup.GET(systemPath+"*", echo.WrapHandler(systemHandler))

	ethPath, ethHandler := ethv1connect.NewEthServiceHandler(eth, connectJSONOpt)
	rpcGroup.POST(ethPath+"*", echo.WrapHandler(ethHandler))
	rpcGroup.GET(ethPath+"*", echo.WrapHandler(ethHandler))

	// REST Routes
	restGroup := e.Group("")
	restGroup.GET("/audio/:cid", stub)
	restGroup.GET("/image/:cid", stub)

	return nil
}

func stub(e echo.Context) error {
	return e.String(200, "unimplemented")
}

// Run starts both HTTP (port 80) and HTTPS (port 443, if enabled) servers.
// Both support HTTP/2 (h2/h2c) for ConnectRPC and gRPC.
func (s *Server) Run() error {
	cfg := s.config.OpenAudio.Server
	httpAddr := fmt.Sprintf("%s:%s", cfg.Hostname, cfg.Port)
	httpsAddr := fmt.Sprintf("%s:%s", cfg.Hostname, cfg.HTTPSPort)

	s.logger.Info("initializing server listeners",
		zap.String("http_addr", httpAddr),
		zap.String("https_addr", httpsAddr),
		zap.Bool("tls_enabled", cfg.TLS.Enabled),
		zap.Bool("self_signed", cfg.TLS.SelfSigned),
		zap.Bool("h2c", cfg.H2C),
	)

	// always start HTTP (port 80)
	eg := new(errgroup.Group)

	eg.Go(func() error {
		h2s := &http2.Server{}
		server := &http.Server{
			Addr:    httpAddr,
			Handler: h2c.NewHandler(s.e, h2s),
		}
		s.logger.Info("HTTP listener started", zap.String("addr", httpAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	// start HTTPS (port 443) if enabled
	if cfg.TLS.Enabled {
		if cfg.TLS.SelfSigned {
			eg.Go(func() error {
				s.logger.Info("HTTPS (self-signed) listener starting", zap.String("addr", httpsAddr))
				return s.runSelfSignedTLS(httpsAddr)
			})
		} else {
			eg.Go(func() error {
				s.logger.Info("HTTPS (Let's Encrypt) listener starting", zap.String("addr", httpsAddr))
				return s.runAutoTLS(httpsAddr)
			})
		}
	}

	return eg.Wait()
}

func (s *Server) runSelfSignedTLS(addr string) error {
	cfg := s.config.OpenAudio.Server
	certDir := cfg.TLS.CertDir
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	certFile := filepath.Join(certDir, "cert.pem")
	keyFile := filepath.Join(certDir, "key.pem")

	// Generate if missing
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		s.logger.Info("generating self-signed certs", zap.String("certDir", certDir))
		certPEM, keyPEM, err := generateSelfSignedCert(cfg.Hostname)
		if err != nil {
			return fmt.Errorf("generate self-signed cert: %w", err)
		}
		if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
			return fmt.Errorf("write cert file: %w", err)
		}
		if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
			return fmt.Errorf("write key file: %w", err)
		}
	}

	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load cert pair: %w", err)
	}

	server := &http.Server{
		Addr: addr,
		TLSConfig: &tls.Config{
			NextProtos:   []string{"h2", "http/1.1"},
			Certificates: []tls.Certificate{tlsCert},
			MinVersion:   tls.VersionTLS12,
		},
		Handler: s.e,
	}
	return server.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) runAutoTLS(addr string) error {
	cfg := s.config.OpenAudio.Server
	cacheDir := cfg.TLS.CacheDir
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	// whitelist current host + IPs
	whitelist := []string{cfg.Hostname, "localhost"}
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip := ipnet.IP.To4(); ip != nil {
				whitelist = append(whitelist, ip.String())
			}
		}
	}

	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(whitelist...),
		Cache:      autocert.DirCache(cacheDir),
	}

	s.logger.Info("Let's Encrypt autocert enabled",
		zap.Strings("hosts", whitelist),
		zap.String("cacheDir", cacheDir),
	)

	s.e.Pre(middleware.HTTPSRedirect())

	server := &http.Server{
		Addr:      addr,
		TLSConfig: manager.TLSConfig(),
		Handler:   s.e,
	}
	return server.ListenAndServeTLS("", "")
}

func generateSelfSignedCert(hostname string) ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certPEM, privateKeyPEM, nil
}
