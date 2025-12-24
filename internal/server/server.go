package server

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/Zereker/memory/internal/action"
	"github.com/Zereker/memory/internal/api/http"
	"github.com/Zereker/memory/internal/api/mcp"
	genkitpkg "github.com/Zereker/memory/pkg/genkit"
	"github.com/Zereker/memory/pkg/log"
	"github.com/Zereker/memory/pkg/relation"
	"github.com/Zereker/memory/pkg/vector"
)

// Server represents the memory server
type Server struct {
	config Config
	logger *slog.Logger
	memory *action.Memory
	store  *vector.OpenSearchStore
}

// NewServer creates a new server with the given configuration
func NewServer(conf Config) (*Server, error) {
	server := &Server{
		config: conf,
	}

	if err := server.initDepend(); err != nil {
		return nil, errors.WithMessage(err, "init server dependency failed")
	}

	if err := server.initMemory(); err != nil {
		return nil, errors.WithMessage(err, "init memory failed")
	}

	return server, nil
}

// initDepend initializes all dependencies
func (s *Server) initDepend() error {
	// Initialize log first
	if err := log.Init(s.config.Log); err != nil {
		return errors.WithMessage(err, "failed to init log")
	}

	// Create logger for this module
	s.logger = log.Logger("server")
	s.logger.Info("initializing dependencies")

	ctx := context.Background()

	// Initialize Genkit with all configured models
	s.logger.Info("initializing genkit models")
	if err := genkitpkg.Init(ctx, s.config.Models); err != nil {
		return errors.WithMessage(err, "failed to init models")
	}

	// Initialize OpenSearch storage singleton
	s.logger.Info("initializing storage")
	if err := vector.Init(s.config.Storage); err != nil {
		return errors.WithMessage(err, "failed to init storage")
	}
	s.store = vector.NewStore()

	// Initialize PostgreSQL relation store
	s.logger.Info("initializing relation store")
	if err := relation.Init(s.config.Postgres); err != nil {
		return errors.WithMessage(err, "failed to init relation store")
	}

	return nil
}

// initMemory initializes the memory instance
func (s *Server) initMemory() error {
	s.logger.Info("initializing memory")
	s.memory = action.NewMemory()
	return nil
}

// Start starts the server based on configuration mode
func (s *Server) Start() error {
	s.logger.Info("starting", "mode", s.config.Server.Mode, "port", s.config.Server.Port)

	ctx, cancel := context.WithCancel(context.Background())

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		s.logger.Info("received shutdown signal")
		cancel()
	}()

	g, ctx := errgroup.WithContext(ctx)

	switch s.config.Server.Mode {
	case "http":
		g.Go(func() error {
			return s.runHTTPServer(ctx)
		})
	case "mcp":
		g.Go(func() error {
			return s.runMCPServer(ctx)
		})
	case "both":
		g.Go(func() error {
			return s.runHTTPServer(ctx)
		})
		g.Go(func() error {
			return s.runMCPServer(ctx)
		})
	default:
		cancel()
		return errors.Errorf("unknown mode: %s", s.config.Server.Mode)
	}

	return g.Wait()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	s.logger.Info("shutting down")

	ctx := context.Background()

	if err := relation.Close(ctx); err != nil {
		s.logger.Error("failed to close relation store", "error", err)
	}

	if s.store != nil {
		_ = s.store.Close()
	}

	return nil
}

func (s *Server) runHTTPServer(ctx context.Context) error {
	serverCfg := http.DefaultServerConfig()
	serverCfg.Port = s.config.Server.Port

	srv := http.NewServer(s.memory, serverCfg)

	// Shutdown when context is cancelled
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	if err := srv.Start(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		return errors.WithMessage(err, "http server error")
	}
	return nil
}

func (s *Server) runMCPServer(ctx context.Context) error {
	server := mcp.NewServer(s.memory, mcp.ServerConfig{
		Name:    "memory",
		Version: "0.1.0",
	})

	if err := server.RunStdio(ctx); err != nil && err != context.Canceled {
		return errors.WithMessage(err, "mcp server error")
	}
	return nil
}
