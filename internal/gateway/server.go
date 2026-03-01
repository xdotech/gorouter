package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/db"
	"github.com/xdotech/gorouter/internal/domain"
	"github.com/xdotech/gorouter/internal/logging"
	"github.com/xdotech/gorouter/internal/oauth"
	"github.com/xdotech/gorouter/internal/router"
	"github.com/xdotech/gorouter/internal/usage"
)

// Server represents the HTTP gateway server.
type Server struct {
	server        *http.Server
	cfg           *config.Config
	stores        *domain.Stores
	dbStore       *db.Store // legacy store — used by router.Handler and oauth
	tracker       *usage.Tracker
	usageDB       *usage.DB
	rh            *router.Handler // AI routing handler
	oh            *oauth.Handler  // OAuth handler
	schedulerDone chan struct{}   // signals scheduler goroutine to stop
}

// NewServer creates a new gateway Server.
func NewServer(cfg *config.Config, stores *domain.Stores, dbStore *db.Store, tracker *usage.Tracker, usageDB *usage.DB) *Server {
	s := &Server{
		cfg:     cfg,
		stores:  stores,
		dbStore: dbStore,
		tracker: tracker,
		usageDB: usageDB,
		rh:      router.NewHandler(dbStore, tracker),
		oh:      oauth.NewHandler(dbStore, cfg),
	}

	handler := s.setupRouter()

	s.server = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	return s
}

// Name implements lifecycle.Service.
func (s *Server) Name() string { return "http" }

// Start implements lifecycle.Service.
func (s *Server) Start(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	// Start background token refresh scheduler.
	s.schedulerDone = make(chan struct{})
	oauth.StartScheduler(s.dbStore, s.schedulerDone)

	logger.Info("starting server", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop implements lifecycle.Service.
func (s *Server) Stop(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Info("shutting down server")

	// Stop background scheduler.
	if s.schedulerDone != nil {
		close(s.schedulerDone)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.server.Shutdown(shutdownCtx)
}
