package main

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "time"

    "github.com/gorilla/mux"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/chesskiss/btc-service/clients"
    "github.com/chesskiss/btc-service/config"
    "github.com/chesskiss/btc-service/handlers"
    "github.com/chesskiss/btc-service/internal/database"
    internalHandlers "github.com/chesskiss/btc-service/internal/handlers"
    "github.com/chesskiss/btc-service/internal/middleware"
    "github.com/chesskiss/btc-service/internal/tracing"
)

func main() {
    // Initialize structured logging (JSON format)
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    slog.Info("starting Bitcoin LTP service")

    cfg := config.Load()

    // Initialize OpenTelemetry tracing
    tp, err := tracing.InitTracer("btc-service")
    if err != nil {
        slog.Error("failed to initialize tracer", "error", err)
        os.Exit(1)
    }
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := tracing.Shutdown(ctx, tp); err != nil {
            slog.Error("failed to shutdown tracer", "error", err)
        }
    }()

    // Initialize Redis
    redisClient := clients.InitRedis(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword)

    // Initialize PostgreSQL
    db, err := database.InitDB(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
    if err != nil {
        slog.Warn("database initialization failed",
            "error", err,
        )
        slog.Info("continuing without request logging")
    }
    defer database.Close()

    // Setup router
    r := mux.NewRouter()

    // Health and readiness checks
    r.HandleFunc("/health", internalHandlers.HealthHandler).Methods("GET")
    r.HandleFunc("/ready", internalHandlers.ReadinessHandler(db, redisClient)).Methods("GET")

    // Prometheus metrics
    r.Handle("/metrics", promhttp.Handler()).Methods("GET")

    // API endpoints
    r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")

    // Apply logging middleware
    handler := middleware.LoggingMiddleware(r)

    // Start server
    addr := fmt.Sprintf(":%s", cfg.Port)
    slog.Info("server starting",
        "address", addr,
    )

    if err := http.ListenAndServe(addr, handler); err != nil {
        slog.Error("server failed",
            "error", err,
        )
        os.Exit(1)
    }
}
