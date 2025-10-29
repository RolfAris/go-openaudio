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
	coreServer "github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/OpenAudio/go-openaudio/pkg/eth"
	mediorumServer "github.com/OpenAudio/go-openaudio/pkg/mediorum/server"
	servermw "github.com/OpenAudio/go-openaudio/pkg/server/middleware"
	"github.com/OpenAudio/go-openaudio/pkg/system"
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

	core    *coreServer.CoreService
	storage *mediorumServer.StorageService
	system  *system.SystemService
	eth     *eth.EthService

	httpServer  *http.Server
	httpsServer *http.Server
}

func NewServer(ctx context.Context, config *config.Config, logger *zap.Logger, core *coreServer.CoreService, storage *mediorumServer.StorageService, systemSvc *system.SystemService, ethSvc *eth.EthService) *Server {
	return &Server{
		ctx:    ctx,
		config: config,
		logger: logger,
		e:      echo.New(),

		core:    core,
		storage: storage,
		system:  systemSvc,
		eth:     ethSvc,
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

	e.Use(servermw.ZapLogger(s.logger))
	e.Use(middleware.CORS())
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

	// Setup HTTP Server
	cfg := s.config.OpenAudio.Server
	httpAddr := fmt.Sprintf("%s:%s", cfg.Hostname, cfg.Port)
	h2s := &http2.Server{}
	s.httpServer = &http.Server{
		Addr:    httpAddr,
		Handler: h2c.NewHandler(s.e, h2s),
	}

	// Setup HTTPS Server (if enabled)
	if cfg.TLS.Enabled {
		httpsAddr := fmt.Sprintf("%s:%s", cfg.Hostname, cfg.HTTPSPort)
		var err error
		if cfg.TLS.SelfSigned {
			s.httpsServer, err = s.setupSelfSignedTLS(httpsAddr)
		} else {
			s.httpsServer, err = s.setupAutoTLS(httpsAddr)
		}
		if err != nil {
			return fmt.Errorf("setup TLS: %w", err)
		}
	}

	return nil
}

func stub(e echo.Context) error {
	return e.String(200, "unimplemented")
}

func (s *Server) Run() error {
	cfg := s.config.OpenAudio.Server

	s.logger.Info("starting server listeners",
		zap.String("http_addr", s.httpServer.Addr),
		zap.Bool("tls_enabled", cfg.TLS.Enabled),
		zap.Bool("h2c", cfg.H2C),
	)

	eg, ctx := errgroup.WithContext(s.ctx)

	// HTTP Server
	s.httpServer.BaseContext = func(net.Listener) context.Context {
		return ctx
	}

	eg.Go(func() error {
		s.logger.Info("HTTP listener started", zap.String("addr", s.httpServer.Addr))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		<-ctx.Done()
		s.logger.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return nil
	})

	// HTTPS Server (optional)
	if s.httpsServer != nil {
		s.httpsServer.BaseContext = func(net.Listener) context.Context {
			return ctx
		}

		eg.Go(func() error {
			s.logger.Info("HTTPS listener started", zap.String("addr", s.httpsServer.Addr))
			if err := s.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("https server: %w", err)
			}
			return nil
		})

		eg.Go(func() error {
			<-ctx.Done()
			s.logger.Info("shutting down HTTPS server")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.httpsServer.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("shutdown https server: %w", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("server crashed: %w", err)
	}

	return nil
}

func (s *Server) setupSelfSignedTLS(addr string) (*http.Server, error) {
	cfg := s.config.OpenAudio.Server
	certDir := cfg.TLS.CertDir
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	certFile := filepath.Join(certDir, "cert.pem")
	keyFile := filepath.Join(certDir, "key.pem")

	// Generate if missing
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		s.logger.Info("generating self-signed certs", zap.String("certDir", certDir))
		certPEM, keyPEM, err := generateSelfSignedCert(cfg.Hostname)
		if err != nil {
			return nil, fmt.Errorf("generate self-signed cert: %w", err)
		}
		if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
			return nil, fmt.Errorf("write cert file: %w", err)
		}
		if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
			return nil, fmt.Errorf("write key file: %w", err)
		}
	}

	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load cert pair: %w", err)
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
	return server, nil
}

func (s *Server) setupAutoTLS(addr string) (*http.Server, error) {
	cfg := s.config.OpenAudio.Server
	cacheDir := cfg.TLS.CacheDir
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
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

	return server, nil
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
