package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"github.com/chesskiss/btc-service/handlers"
	"github.com/chesskiss/btc-service/internal/database"
	"github.com/chesskiss/btc-service/internal/middleware"
)

// setupIntegrationDB creates a test database for integration tests
func setupIntegrationDB(t *testing.T) *sql.DB {
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=btc_service_test sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping integration tests: PostgreSQL not available: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration tests: PostgreSQL not reachable: %v", err)
		return nil
	}

	// Create test database schema
	createSchema := `
		DROP TABLE IF EXISTS request_logs;
		CREATE TABLE request_logs (
			id SERIAL PRIMARY KEY,
			request_id VARCHAR(36) UNIQUE,
			timestamp TIMESTAMP DEFAULT NOW(),
			method VARCHAR(10),
			endpoint VARCHAR(100),
			pairs_requested TEXT,
			user_ip VARCHAR(45),
			status_code INT,
			response_time_ms INT,
			cache_hit BOOLEAN,
			kraken_calls INT,
			error_occurred BOOLEAN,
			error_message TEXT
		);
		CREATE INDEX idx_timestamp ON request_logs(timestamp);
		CREATE INDEX idx_status ON request_logs(status_code);
	`

	_, err = db.Exec(createSchema)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	return db
}

// cleanupIntegrationDB removes all data from the test database
func cleanupIntegrationDB(t *testing.T, db *sql.DB) {
	if db != nil {
		_, err := db.Exec("TRUNCATE TABLE request_logs")
		if err != nil {
			t.Logf("Warning: Failed to cleanup test database: %v", err)
		}
		db.Close()
	}
}

// waitForAsyncLog waits for asynchronous database logging to complete
func waitForAsyncLog() {
	time.Sleep(100 * time.Millisecond)
}

func TestDatabaseIntegration_SuccessfulRequest(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	// Initialize database package
	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	// Create router with middleware
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	// Make request
	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Wait for async logging
	waitForAsyncLog()

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify database logging
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 logged request, got %d", count)
	}

	// Verify logged data
	var reqLog database.RequestLog
	err = db.QueryRow(`
		SELECT request_id, method, endpoint, pairs_requested, user_ip,
		       status_code, cache_hit, error_occurred
		FROM request_logs
		LIMIT 1
	`).Scan(
		&reqLog.RequestID,
		&reqLog.Method,
		&reqLog.Endpoint,
		&reqLog.PairsRequested,
		&reqLog.UserIP,
		&reqLog.StatusCode,
		&reqLog.CacheHit,
		&reqLog.ErrorOccurred,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve logged request: %v", err)
	}

	// Verify request details
	if reqLog.Method != "GET" {
		t.Errorf("Expected method GET, got %s", reqLog.Method)
	}
	if reqLog.Endpoint != "/api/v1/ltp" {
		t.Errorf("Expected endpoint /api/v1/ltp, got %s", reqLog.Endpoint)
	}
	if reqLog.PairsRequested != "BTC/USD" {
		t.Errorf("Expected pairs BTC/USD, got %s", reqLog.PairsRequested)
	}
	if reqLog.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", reqLog.StatusCode)
	}
	if reqLog.ErrorOccurred {
		t.Errorf("Expected error_occurred to be false")
	}
}

func TestDatabaseIntegration_MultiplePairs(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	// Request multiple pairs
	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD,BTC/EUR,BTC/GBP", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.45")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	waitForAsyncLog()

	// Verify pairs were logged
	var pairsRequested string
	err = db.QueryRow("SELECT pairs_requested FROM request_logs LIMIT 1").Scan(&pairsRequested)
	if err != nil {
		t.Fatalf("Failed to retrieve pairs_requested: %v", err)
	}

	if pairsRequested != "BTC/USD,BTC/EUR,BTC/GBP" {
		t.Errorf("Expected pairs 'BTC/USD,BTC/EUR,BTC/GBP', got '%s'", pairsRequested)
	}

	// Verify user IP from X-Forwarded-For
	var userIP string
	err = db.QueryRow("SELECT user_ip FROM request_logs LIMIT 1").Scan(&userIP)
	if err != nil {
		t.Fatalf("Failed to retrieve user_ip: %v", err)
	}

	if userIP != "203.0.113.45" {
		t.Errorf("Expected user_ip '203.0.113.45', got '%s'", userIP)
	}
}

func TestDatabaseIntegration_MultipleRequests(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	// Make multiple requests
	numRequests := 5
	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/ltp?pairs=BTC/USD"), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	waitForAsyncLog()

	// Verify all requests were logged
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != numRequests {
		t.Errorf("Expected %d logged requests, got %d", numRequests, count)
	}

	// Verify all have unique request IDs
	var uniqueCount int
	err = db.QueryRow("SELECT COUNT(DISTINCT request_id) FROM request_logs").Scan(&uniqueCount)
	if err != nil {
		t.Fatalf("Failed to query unique request IDs: %v", err)
	}

	if uniqueCount != numRequests {
		t.Errorf("Expected %d unique request IDs, got %d", numRequests, uniqueCount)
	}
}

func TestDatabaseIntegration_RequestIDPropagation(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	waitForAsyncLog()

	// Verify request_id is a valid UUID format
	var requestID string
	err = db.QueryRow("SELECT request_id FROM request_logs LIMIT 1").Scan(&requestID)
	if err != nil {
		t.Fatalf("Failed to retrieve request_id: %v", err)
	}

	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (36 characters with hyphens)
	if len(requestID) != 36 {
		t.Errorf("Expected request_id to be 36 characters (UUID format), got %d: %s", len(requestID), requestID)
	}

	// Basic validation that it contains hyphens in right positions
	if requestID[8] != '-' || requestID[13] != '-' || requestID[18] != '-' || requestID[23] != '-' {
		t.Errorf("request_id doesn't match UUID format: %s", requestID)
	}
}

func TestDatabaseIntegration_ResponseTimeTracking(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	waitForAsyncLog()

	// Verify response time is tracked
	var responseTimeMs int
	err = db.QueryRow("SELECT response_time_ms FROM request_logs LIMIT 1").Scan(&responseTimeMs)
	if err != nil {
		t.Fatalf("Failed to retrieve response_time_ms: %v", err)
	}

	// Response time should be positive and reasonable (less than 10 seconds for tests)
	if responseTimeMs < 0 {
		t.Errorf("Expected positive response time, got %d", responseTimeMs)
	}
	if responseTimeMs > 10000 {
		t.Errorf("Response time seems too high: %d ms", responseTimeMs)
	}
}

func TestDatabaseIntegration_WithoutDatabaseConnection(t *testing.T) {
	// Close database connection to simulate failure
	database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)
	w := httptest.NewRecorder()

	// Request should still succeed even without database
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected request to succeed without database, got status %d", w.Code)
	}

	// Verify response is valid JSON
	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Errorf("Expected valid JSON response, got error: %v", err)
	}
}

func TestDatabaseIntegration_IPAddressExtraction(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	tests := []struct {
		name           string
		headerName     string
		headerValue    string
		expectedIP     string
		useRemoteAddr  bool
		remoteAddrVal  string
	}{
		{
			name:        "X-Forwarded-For single IP",
			headerName:  "X-Forwarded-For",
			headerValue: "203.0.113.10",
			expectedIP:  "203.0.113.10",
		},
		{
			name:        "X-Forwarded-For multiple IPs",
			headerName:  "X-Forwarded-For",
			headerValue: "203.0.113.10, 198.51.100.5",
			expectedIP:  "203.0.113.10",
		},
		{
			name:        "X-Real-IP",
			headerName:  "X-Real-IP",
			headerValue: "198.51.100.20",
			expectedIP:  "198.51.100.20",
		},
		{
			name:          "RemoteAddr fallback",
			useRemoteAddr: true,
			remoteAddrVal: "192.0.2.30:54321",
			expectedIP:    "192.0.2.30",
		},
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up previous test data
			_, _ = db.Exec("TRUNCATE TABLE request_logs")

			req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)

			if tt.headerName != "" {
				req.Header.Set(tt.headerName, tt.headerValue)
			}
			if tt.useRemoteAddr {
				req.RemoteAddr = tt.remoteAddrVal
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			waitForAsyncLog()

			var userIP string
			err := db.QueryRow("SELECT user_ip FROM request_logs LIMIT 1").Scan(&userIP)
			if err != nil {
				t.Fatalf("Failed to retrieve user_ip: %v", err)
			}

			if userIP != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, userIP)
			}
		})
	}
}

func TestDatabaseIntegration_EmptyPairsParameter(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	// Request without pairs parameter
	req := httptest.NewRequest("GET", "/api/v1/ltp", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	waitForAsyncLog()

	// Verify pairs_requested is empty string (not null)
	var pairsRequested string
	var found bool
	err = db.QueryRow("SELECT pairs_requested FROM request_logs LIMIT 1").Scan(&pairsRequested)
	if err != nil {
		t.Fatalf("Failed to retrieve pairs_requested: %v", err)
	}

	found = true
	if !found {
		t.Errorf("Expected to find a request log entry")
	}

	// Verify it's an empty string
	if pairsRequested != "" {
		t.Errorf("Expected empty pairs_requested, got '%s'", pairsRequested)
	}
}

func TestDatabaseIntegration_IndexesExist(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	// Verify that indexes exist for query optimization
	indexQueries := []struct {
		indexName string
		query     string
	}{
		{
			"idx_timestamp",
			"SELECT 1 FROM pg_indexes WHERE indexname = 'idx_timestamp'",
		},
		{
			"idx_status",
			"SELECT 1 FROM pg_indexes WHERE indexname = 'idx_status'",
		},
	}

	for _, idx := range indexQueries {
		var exists int
		err := db.QueryRow(idx.query).Scan(&exists)
		if err != nil {
			t.Errorf("Index %s does not exist: %v", idx.indexName, err)
		}
	}
}

func TestDatabaseIntegration_ErrorLogging(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	// Request with invalid pair that will cause Kraken API error
	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/INVALID", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	waitForAsyncLog()

	// Verify error was logged
	var errorOccurred bool
	var errorMessage string
	var statusCode int
	var krakenCalls int
	err = db.QueryRow(`
		SELECT error_occurred, error_message, status_code, kraken_calls
		FROM request_logs
		LIMIT 1
	`).Scan(&errorOccurred, &errorMessage, &statusCode, &krakenCalls)
	if err != nil {
		t.Fatalf("Failed to retrieve error log: %v", err)
	}

	if !errorOccurred {
		t.Errorf("Expected error_occurred to be true for invalid pair")
	}

	if errorMessage == "" {
		t.Errorf("Expected error_message to be non-empty")
	}

	if statusCode != 503 {
		t.Errorf("Expected status code 503 (service unavailable), got %d", statusCode)
	}

	if krakenCalls != 1 {
		t.Errorf("Expected 1 Kraken call, got %d", krakenCalls)
	}

	// Verify error message contains meaningful information
	if len(errorMessage) < 5 {
		t.Errorf("Error message seems too short: '%s'", errorMessage)
	}
}

func TestDatabaseIntegration_KrakenCallsTracking(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	testCases := []struct {
		name                string
		pairs               string
		expectedKrakenCalls int
	}{
		{"Single pair", "BTC/USD", 1},
		{"Two pairs", "BTC/USD,BTC/EUR", 2},
		{"Three pairs", "BTC/USD,BTC/EUR,BTC/GBP", 3},
		{"Empty (defaults to 3)", "", 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up previous test data
			_, _ = db.Exec("TRUNCATE TABLE request_logs")

			url := "/api/v1/ltp"
			if tc.pairs != "" {
				url += "?pairs=" + tc.pairs
			}

			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			waitForAsyncLog()

			var krakenCalls int
			err := db.QueryRow("SELECT kraken_calls FROM request_logs LIMIT 1").Scan(&krakenCalls)
			if err != nil {
				t.Fatalf("Failed to retrieve kraken_calls: %v", err)
			}

			if krakenCalls != tc.expectedKrakenCalls {
				t.Errorf("Expected %d Kraken calls, got %d", tc.expectedKrakenCalls, krakenCalls)
			}
		})
	}
}

func TestDatabaseIntegration_PartialFailure(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	// Request with mix of valid and invalid pairs
	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD,BTC/INVALID", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	waitForAsyncLog()

	var errorOccurred bool
	var statusCode int
	var krakenCalls int
	err = db.QueryRow(`
		SELECT error_occurred, status_code, kraken_calls
		FROM request_logs
		LIMIT 1
	`).Scan(&errorOccurred, &statusCode, &krakenCalls)
	if err != nil {
		t.Fatalf("Failed to retrieve log: %v", err)
	}

	// Partial failure: error occurred but should still return 200
	// because we got some successful results
	if !errorOccurred {
		t.Errorf("Expected error_occurred to be true for partial failure")
	}

	if statusCode != 200 {
		t.Errorf("Expected status code 200 for partial success, got %d", statusCode)
	}

	if krakenCalls != 2 {
		t.Errorf("Expected 2 Kraken calls (one for each pair), got %d", krakenCalls)
	}
}

func TestDatabaseIntegration_QueryByTimestamp(t *testing.T) {
	db := setupIntegrationDB(t)
	if db == nil {
		return
	}
	defer cleanupIntegrationDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		time.Sleep(10 * time.Millisecond)
	}

	waitForAsyncLog()

	// Query recent requests
	rows, err := db.Query(`
		SELECT request_id, timestamp
		FROM request_logs
		ORDER BY timestamp DESC
		LIMIT 10
	`)
	if err != nil {
		t.Fatalf("Failed to query by timestamp: %v", err)
	}
	defer rows.Close()

	var count int
	var prevTimestamp time.Time
	for rows.Next() {
		var requestID string
		var timestamp time.Time
		if err := rows.Scan(&requestID, &timestamp); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		count++

		// Verify descending order
		if count > 1 && timestamp.After(prevTimestamp) {
			t.Errorf("Timestamps not in descending order")
		}
		prevTimestamp = timestamp
	}

	if count != 3 {
		t.Errorf("Expected 3 records, got %d", count)
	}
}
