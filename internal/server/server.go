package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"internal-transfers/internal/config"
	"internal-transfers/internal/handler"
	"internal-transfers/internal/repository"
	"internal-transfers/internal/service"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// Server represents the HTTP server
type Server struct {
	router *mux.Router
	server *http.Server
	db     *sql.DB
	logger *slog.Logger
	port   string
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	// Initialize database connection
	db, err := sql.Open("postgres", cfg.GetDBConnectionString())
	if err != nil {
		return nil, err
	}

	// Configure connection pool for better performance
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test database connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	if logger != nil {
		logger.Info("Successfully connected to database")
	}

	// Initialize store (Unit of Work)
	store := repository.NewStore(db, logger)

	// Initialize services
	accountService := service.NewAccountService(store, logger)
	transactionService := service.NewTransactionService(store, logger)

	// Initialize handlers
	accountHandler := handler.NewAccountHandler(accountService)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	// Setup router
	router := mux.NewRouter()

	// Add middleware for logging
	router.Use(loggingMiddleware(logger))

	// Account routes
	router.HandleFunc("/accounts", accountHandler.CreateAccount).Methods("POST")
	router.HandleFunc("/accounts/{account_id}", accountHandler.GetAccount).Methods("GET")

	// Transaction routes
	router.HandleFunc("/transactions", transactionHandler.Transfer).Methods("POST")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check database connectivity in health check
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": "database unavailable"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}).Methods("GET")

	return &Server{
		router: router,
		db:     db,
		logger: logger,
	}, nil
}

// loggingMiddleware adds request logging
func loggingMiddleware(logger *slog.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create response wrapper to capture status code
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.statusCode,
				"duration", time.Since(start),
				"user_agent", r.UserAgent(),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Start starts the HTTP server on the specified port
func (s *Server) Start(port string) (string, error) {
	// Create listener first to get actual port
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return "", err
	}

	// Get the actual port being used
	addr := listener.Addr().(*net.TCPAddr)
	s.port = strconv.Itoa(addr.Port)

	// Create HTTP server
	s.server = &http.Server{
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if s.logger != nil {
		s.logger.Info("Starting server", "port", s.port)
	}

	// Start server in background
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			if s.logger != nil {
				s.logger.Error("Server failed to start", "error", err)
			}
		}
	}()

	return s.port, nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info("Shutting down server")
	}

	// Close database connection
	if s.db != nil {
		s.db.Close()
	}

	// Shutdown HTTP server
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// GetPort returns the port the server is listening on
func (s *Server) GetPort() string {
	return s.port
}

// GetBaseURL returns the base URL for the server
func (s *Server) GetBaseURL() string {
	return "http://localhost:" + s.port
}

// GetRouter returns the router for testing purposes
func (s *Server) GetRouter() *mux.Router {
	return s.router
}

// StartServer starts the server with the given configuration
func StartServer(cfg *config.Config) (*Server, string, error) {
	// Initialize logger - use io.Discard for tests to avoid panic
	var logger *slog.Logger
	if cfg.ServerPort == "0" {
		// Test environment - use discard logger
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	} else {
		// Production environment - use stdout
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	server, err := NewServer(cfg, logger)
	if err != nil {
		return nil, "", err
	}

	// Start the server and get the actual port
	port, err := server.Start(cfg.ServerPort)
	if err != nil {
		return nil, "", err
	}

	return server, port, nil
}
