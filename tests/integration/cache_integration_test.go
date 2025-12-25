package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/chesskiss/btc-service/clients"
	"github.com/chesskiss/btc-service/services"
	"github.com/redis/go-redis/v9"
)

func setupIntegrationRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0, // Use same DB as the actual client
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Skip("Redis not available for integration testing")
	}

	client.FlushDB(ctx)
	return client
}

func TestCacheIntegrationFullFlow(t *testing.T) {
	redisClient := setupIntegrationRedis(t)
	defer redisClient.Close()

	clients.InitRedis("localhost", "6379", "")
	server := createTestServer()
	defer server.Close()

	// First request - should cache
	resp1, err := http.Get(server.URL + "/api/v1/ltp?pairs=BTC/USD")
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	defer resp1.Body.Close()

	var ltpResp1 services.LTPResponse
	if err := json.NewDecoder(resp1.Body).Decode(&ltpResp1); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	if len(ltpResp1.LTP) == 0 {
		t.Fatal("expected at least one price")
	}

	price1 := ltpResp1.LTP[0].Amount

	// Verify cache was set
	ctx := context.Background()
	exists, _ := redisClient.Exists(ctx, "price:BTC/USD").Result()
	if exists != 1 {
		t.Error("expected cache to be set after first request")
	}

	// Second request - should use cache
	resp2, err := http.Get(server.URL + "/api/v1/ltp?pairs=BTC/USD")
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	defer resp2.Body.Close()

	var ltpResp2 services.LTPResponse
	if err := json.NewDecoder(resp2.Body).Decode(&ltpResp2); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	price2 := ltpResp2.LTP[0].Amount

	if price1 != price2 {
		t.Errorf("cached price %f should match first price %f", price2, price1)
	}
}

func TestCacheIntegrationMultiplePairs(t *testing.T) {
	redisClient := setupIntegrationRedis(t)
	defer redisClient.Close()

	clients.InitRedis("localhost", "6379", "")
	server := createTestServer()
	defer server.Close()

	// Request multiple pairs
	resp, err := http.Get(server.URL + "/api/v1/ltp?pairs=BTC/USD,BTC/EUR,BTC/CHF")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var ltpResp services.LTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&ltpResp); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// Verify all pairs are cached
	ctx := context.Background()
	expectedKeys := []string{"price:BTC/USD", "price:BTC/EUR", "price:BTC/CHF"}
	for _, key := range expectedKeys {
		exists, err := redisClient.Exists(ctx, key).Result()
		if err != nil {
			t.Fatalf("failed to check key %s: %v", key, err)
		}
		if exists != 1 {
			t.Errorf("expected cache key %s to exist", key)
		}
	}
}

func TestCacheIntegrationWithoutRedis(t *testing.T) {
	t.Skip("Skipping test that pollutes global Redis client state")
	// This test is problematic because it sets the global redisClient to an invalid host,
	// which affects subsequent tests. The graceful degradation is already tested in unit tests.
}

func TestCacheIntegrationExpiry(t *testing.T) {
	redisClient := setupIntegrationRedis(t)
	defer redisClient.Close()

	clients.InitRedis("localhost", "6379", "")

	// Get price to cache it
	_, err := clients.GetBTCPrice(context.Background(), "USD")
	if err != nil {
		t.Fatalf("failed to get price: %v", err)
	}

	// Verify cache exists with proper TTL
	ctx := context.Background()
	ttl, err := redisClient.TTL(ctx, "price:BTC/USD").Result()
	if err != nil {
		t.Fatalf("failed to get TTL: %v", err)
	}

	if ttl <= 0 {
		t.Error("expected positive TTL")
	}

	if ttl > 60*time.Second {
		t.Errorf("TTL %v exceeds 60 seconds", ttl)
	}

	// Manually set TTL to 1 second for faster testing
	redisClient.Expire(ctx, "price:BTC/USD", 1*time.Second)

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Key should be gone
	exists, err := redisClient.Exists(ctx, "price:BTC/USD").Result()
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}

	if exists != 0 {
		t.Error("expected cache key to be expired")
	}
}

func TestCacheIntegrationConcurrentRequests(t *testing.T) {
	redisClient := setupIntegrationRedis(t)
	defer redisClient.Close()

	clients.InitRedis("localhost", "6379", "")
	server := createTestServer()
	defer server.Close()

	// Make concurrent requests
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			resp, err := http.Get(server.URL + "/api/v1/ltp?pairs=BTC/USD")
			if err != nil {
				t.Errorf("concurrent request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
			}

			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify cache exists
	ctx := context.Background()
	exists, _ := redisClient.Exists(ctx, "price:BTC/USD").Result()
	if exists != 1 {
		t.Error("expected cache to be set after concurrent requests")
	}
}
