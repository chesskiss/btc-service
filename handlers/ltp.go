package handlers

import (
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "strings"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"

    "github.com/chesskiss/btc-service/internal/database"
    "github.com/chesskiss/btc-service/internal/metrics"
    "github.com/chesskiss/btc-service/internal/middleware"
    "github.com/chesskiss/btc-service/services"
)

var cacheHits int
var krakenCalls int

func LTPHandler(w http.ResponseWriter, r *http.Request) {
    // Start tracing span
    tracer := otel.Tracer("btc-service")
    ctx, span := tracer.Start(r.Context(), "handle_ltp_request")
    defer span.End()

    startTime := time.Now()
    requestID := middleware.GetRequestID(ctx)

    w.Header().Set("Content-Type", "application/json")

    pairsParam := r.URL.Query().Get("pairs")

    // Add span attributes
    span.SetAttributes(
        attribute.String("http.method", r.Method),
        attribute.String("http.url", r.URL.String()),
        attribute.String("http.route", r.URL.Path),
        attribute.String("request.id", requestID),
        attribute.String("request.pairs", pairsParam),
    )

    slog.Info("fetching prices",
        "request_id", requestID,
        "pairs", pairsParam,
    )

    result := services.GetPrices(ctx, pairsParam)

    // Calculate response time
    duration := time.Since(startTime)
    responseTime := int(duration.Milliseconds())

    // Determine if error occurred (all requests failed or partial failure)
    totalRequests := result.KrakenCalls
    successCount := len(result.Prices)
    errorOccurred := result.ErrorsCount > 0

    // Determine cache hit (if any prices were returned and response was fast)
    cacheHit := successCount > 0 && responseTime < 100

    // Get client IP
    userIP := getClientIP(r)

    // Determine HTTP status code
    statusCode := http.StatusOK
    if errorOccurred && successCount == 0 {
        // All requests failed - service unavailable
        statusCode = http.StatusServiceUnavailable
    } else if errorOccurred {
        // Partial failure - still return 200 with partial data
        statusCode = http.StatusOK
    }

    // Add more span attributes with results
    span.SetAttributes(
        attribute.Int("http.status_code", statusCode),
        attribute.Int("response.pairs_count", successCount),
        attribute.Int("response.errors_count", result.ErrorsCount),
        attribute.Bool("response.cache_hit", cacheHit),
        attribute.Int("response.kraken_calls", totalRequests),
        attribute.Int("response.time_ms", responseTime),
    )

    // Set span status based on errors
    if errorOccurred && successCount == 0 {
        span.SetStatus(codes.Error, "all price fetches failed")
        span.RecordError(fmt.Errorf("%s", result.ErrorMessage))
    } else if errorOccurred {
        span.SetStatus(codes.Ok, "partial success")
    } else {
        span.SetStatus(codes.Ok, "success")
    }

    // Record metrics
    metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", statusCode)).Inc()
    metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())

    slog.Info("prices fetched",
        "request_id", requestID,
        "pairs_count", successCount,
        "errors_count", result.ErrorsCount,
        "cache_hit", cacheHit,
        "duration_ms", responseTime,
    )

    // Log request to database (don't fail if DB is down)
    go func() {
        _ = database.LogRequest(database.RequestLog{
            RequestID:      requestID,
            Method:         r.Method,
            Endpoint:       r.URL.Path,
            PairsRequested: pairsParam,
            UserIP:         userIP,
            StatusCode:     statusCode,
            ResponseTimeMs: responseTime,
            CacheHit:       cacheHit,
            KrakenCalls:    totalRequests,
            ErrorOccurred:  errorOccurred,
            ErrorMessage:   result.ErrorMessage,
        })
    }()

    // Set response status
    w.WriteHeader(statusCode)

    // Return response
    json.NewEncoder(w).Encode(
        services.LTPResponse{LTP: result.Prices},
    )
}

func getClientIP(r *http.Request) string {
    // Check X-Forwarded-For header first (for proxied requests)
    if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
        ips := strings.Split(forwarded, ",")
        return strings.TrimSpace(ips[0])
    }

    // Check X-Real-IP header
    if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
        return realIP
    }

    // Fall back to RemoteAddr
    ip := r.RemoteAddr
    if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
        ip = ip[:colonIndex]
    }

    return ip
}
