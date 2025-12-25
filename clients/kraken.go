package clients

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"

    "github.com/chesskiss/btc-service/internal/metrics"
    "github.com/redis/go-redis/v9"
)

// For encoding/decoding Kraken JSON
type KrakenResponse struct {
    Error  []string              `json:"error"`
    Result map[string]KrakenPair `json:"result"`
}

type KrakenPair struct {
    C []string `json:"c"` // last trade closed: [price, lot volume]
}

// CachedPrice represents cached price data
type CachedPrice struct {
    Price     float64   `json:"price"`
    Timestamp time.Time `json:"timestamp"`
}

var redisClient *redis.Client
var ctx = context.Background()

// InitRedis initializes the Redis client
func InitRedis(host, port, password string) *redis.Client {
    redisClient = redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%s", host, port),
        Password: password,
        DB:       0,
    })

    // Test connection
    _, err := redisClient.Ping(ctx).Result()
    if err != nil {
        slog.Warn("failed to connect to Redis",
            "error", err,
        )
        slog.Info("continuing without cache")
    } else {
        slog.Info("Redis connected successfully")
    }

    return redisClient
}

// GetBTCPrice fetches the BTC price in the given currency from Kraken API
// with Redis caching support
func GetBTCPrice(ctx context.Context, currency string) (float64, error) {
    tracer := otel.Tracer("btc-service")
    ctx, span := tracer.Start(ctx, "get_btc_price")
    defer span.End()

    pair := fmt.Sprintf("BTC/%s", currency)
    cacheKey := fmt.Sprintf("price:%s", pair)

    span.SetAttributes(
        attribute.String("currency", currency),
        attribute.String("pair", pair),
        attribute.String("cache_key", cacheKey),
    )

    // Try to get from cache first
    if redisClient != nil {
        _, cacheSpan := tracer.Start(ctx, "check_cache")
        cachedPrice, err := getFromCache(cacheKey)
        cacheSpan.End()

        if err == nil && isCacheFresh(cachedPrice) {
            slog.Info("cache hit",
                "pair", pair,
                "price", cachedPrice.Price,
            )
            metrics.CacheHitsTotal.Inc()
            span.SetAttributes(
                attribute.Bool("cache_hit", true),
                attribute.Float64("price", cachedPrice.Price),
            )
            span.SetStatus(codes.Ok, "cache hit")
            return cachedPrice.Price, nil
        }
        if err != nil && err != redis.Nil {
            slog.Warn("cache read error",
                "key", cacheKey,
                "error", err,
            )
        }
    }

    // Cache miss - fetch from Kraken API
    metrics.CacheMissesTotal.Inc()
    slog.Info("cache miss, fetching from Kraken",
        "pair", pair,
    )

    span.SetAttributes(attribute.Bool("cache_hit", false))

    _, krakenSpan := tracer.Start(ctx, "fetch_from_kraken")
    krakenSpan.SetAttributes(
        attribute.String("pair", pair),
        attribute.String("currency", currency),
    )
    price, err := fetchFromKraken(currency)
    if err != nil {
        metrics.KrakenAPIErrorsTotal.Inc()
        slog.Error("kraken API error",
            "pair", pair,
            "error", err,
        )
        krakenSpan.SetStatus(codes.Error, "kraken API error")
        krakenSpan.RecordError(err)
        krakenSpan.End()
        span.SetStatus(codes.Error, "failed to fetch price")
        span.RecordError(err)
        return 0, err
    }

    metrics.KrakenAPICallsTotal.Inc()
    krakenSpan.SetAttributes(attribute.Float64("price", price))
    krakenSpan.SetStatus(codes.Ok, "success")
    krakenSpan.End()

    // Cache the result
    if redisClient != nil {
        if err := saveToCache(cacheKey, price); err != nil {
            slog.Warn("cache write error",
                "key", cacheKey,
                "error", err,
            )
        }
    }

    span.SetAttributes(attribute.Float64("price", price))
    span.SetStatus(codes.Ok, "success")
    return price, nil
}

// getFromCache retrieves cached price data from Redis
func getFromCache(key string) (*CachedPrice, error) {
    val, err := redisClient.Get(ctx, key).Result()
    if err != nil {
        return nil, err
    }

    var cached CachedPrice
    if err := json.Unmarshal([]byte(val), &cached); err != nil {
        return nil, fmt.Errorf("failed to unmarshal cached data: %w", err)
    }

    return &cached, nil
}

// isCacheFresh checks if cached data is less than 60 seconds old
func isCacheFresh(cached *CachedPrice) bool {
    return time.Since(cached.Timestamp) < 60*time.Second
}

// saveToCache stores price data in Redis with 60-second TTL
func saveToCache(key string, price float64) error {
    cached := CachedPrice{
        Price:     price,
        Timestamp: time.Now(),
    }

    data, err := json.Marshal(cached)
    if err != nil {
        return fmt.Errorf("failed to marshal cache data: %w", err)
    }

    slog.Debug("saving to cache",
        "key", key,
        "price", price,
    )

    return redisClient.Set(ctx, key, data, 60*time.Second).Err()
}

// fetchFromKraken fetches price from Kraken API
func fetchFromKraken(currency string) (float64, error) {
    pair := fmt.Sprintf("XBT%s", currency)
    url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pair)

    resp, err := http.Get(url)
    if err != nil {
        return 0, fmt.Errorf("failed to make request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return 0, fmt.Errorf("failed to read response: %w", err)
    }

    var krakenResp KrakenResponse
    if err := json.Unmarshal(body, &krakenResp); err != nil {
        return 0, fmt.Errorf("failed to parse response: %w", err)
    }

    if len(krakenResp.Error) > 0 {
        return 0, fmt.Errorf("kraken API error: %v", krakenResp.Error)
    }

    for _, pairData := range krakenResp.Result {
        if len(pairData.C) > 0 {
            var price float64
            if _, err := fmt.Sscanf(pairData.C[0], "%f", &price); err != nil {
                return 0, fmt.Errorf("failed to parse price: %w", err)
            }
            return price, nil
        }
    }

    return 0, fmt.Errorf("no price data found")
}
