package builder

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"workflow-code-test/api/pkg/cache"
	"workflow-code-test/api/pkg/db"
	"workflow-code-test/api/services/workflow"
)

// Config holds all configuration for the application
type Config struct {
	DatabaseURL     string
	RedisURL        string
	ServerPort      string
	FrontendURL     string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
}

// App represents the application with all its dependencies
type App struct {
	Config          *Config
	Logger          *slog.Logger
	DBPool          *pgxpool.Pool
	Cache           cache.Cache
	Router          *mux.Router
	Server          *http.Server
	WorkflowService *workflow.Service
}

// NewConfig creates a new configuration from environment variables
func NewConfig() (*Config, error) {
	dbURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	// Redis URL is optional - cache will be disabled if not set
	redisURL := os.Getenv("REDIS_URL")

	// Set defaults that can be overridden by env vars
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3003"
	}

	logLevel := slog.LevelDebug
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		switch level {
		case "DEBUG":
			logLevel = slog.LevelDebug
		case "INFO":
			logLevel = slog.LevelInfo
		case "WARN":
			logLevel = slog.LevelWarn
		case "ERROR":
			logLevel = slog.LevelError
		}
	}

	return &Config{
		DatabaseURL:     dbURL,
		RedisURL:        redisURL,
		ServerPort:      serverPort,
		FrontendURL:     frontendURL,
		LogLevel:        logLevel,
		ShutdownTimeout: 5 * time.Second,
	}, nil
}

// SetupLogger configures the application logger
func SetupLogger(level slog.Level) *slog.Logger {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(logHandler)
	slog.SetDefault(logger)
	return logger
}

// SetupDatabase establishes a connection to the database
func SetupDatabase(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return pool, nil
}

// SetupRouter creates and configures the main router
func SetupRouter() *mux.Router {
	mainRouter := mux.NewRouter()
	return mainRouter
}

// SetupServices initializes all application services
func SetupServices(pool *pgxpool.Pool, cacheClient cache.Cache, router *mux.Router) (*workflow.Service, error) {
	// Setup API subrouter
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// Initialize workflow service
	workflowService, err := workflow.NewService(pool, cacheClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow service: %w", err)
	}

	// Load routes
	workflowService.LoadRoutes(apiRouter)

	return workflowService, nil
}

// SetupServer creates and configures the HTTP server
func SetupServer(config *Config, router *mux.Router) *http.Server {
	// Setup CORS
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{config.FrontendURL}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		handlers.AllowCredentials(),
	)(router)

	return &http.Server{
		Addr:              ":" + config.ServerPort,
		Handler:           corsHandler,
		ReadHeaderTimeout: 1 * time.Minute,
	}
}

// Build creates and initializes the entire application
func Build(ctx context.Context) (*App, error) {
	// Load configuration
	config, err := NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logger
	logger := SetupLogger(config.LogLevel)
	logger.Info("Starting application", "port", config.ServerPort)

	// Setup database
	pool, err := SetupDatabase(ctx, config.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		return nil, err
	}

	// Setup cache (optional)
	var cacheClient cache.Cache
	if config.RedisURL == "" {
		logger.Error("Redis URL not configured")
		return nil, fmt.Errorf("redis URL not configured")
	}
	cacheClient, err = cache.NewRedisCache(config.RedisURL)
	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		return nil, err
	}
	logger.Info("Redis cache connected successfully")

	// Setup router
	router := SetupRouter()

	// Setup services
	workflowService, err := SetupServices(pool, cacheClient, router)
	if err != nil {
		logger.Error("Failed to setup services", "error", err)
		pool.Close()
		if err := cacheClient.Close(); err != nil {
			logger.Error("Failed to close cache", "error", err)
		}
		return nil, err
	}

	// Setup server
	server := SetupServer(config, router)

	return &App{
		Config:          config,
		Logger:          logger,
		DBPool:          pool,
		Cache:           cacheClient,
		Router:          router,
		Server:          server,
		WorkflowService: workflowService,
	}, nil
}

// Run starts the application and handles graceful shutdown
func (app *App) Run(ctx context.Context) error {
	// Channel for server errors
	serverErrors := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		app.Logger.Info("Starting server", "address", app.Server.Addr)
		serverErrors <- app.Server.ListenAndServe()
	}()

	// Setup shutdown signal handling
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Wait for either an error or shutdown signal
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		app.Logger.Info("Shutdown signal received", "signal", sig)
		return app.Shutdown(ctx)
	}
}

// Shutdown gracefully shuts down the application
func (app *App) Shutdown(ctx context.Context) error {
	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, app.Config.ShutdownTimeout)
	defer cancel()

	// Shutdown the HTTP server
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		app.Logger.Error("Could not stop server gracefully", "error", err)
		if err := app.Server.Close(); err != nil {
			app.Logger.Error("close HTTP server error", "error", err)
		}
	}

	// Close cache connection
	if app.Cache != nil {
		if err := app.Cache.Close(); err != nil {
			app.Logger.Error("Failed to close cache connection", "error", err)
		}
	}

	// Close database connection
	app.DBPool.Close()

	app.Logger.Info("Application shutdown complete")
	return nil
}

// Close releases all resources
func (app *App) Close() {
	if app.Cache != nil {
		if err := app.Cache.Close(); err != nil {
			slog.Error("Failed to close cache connection")
		}
	}
	if app.DBPool != nil {
		app.DBPool.Close()
	}
}
