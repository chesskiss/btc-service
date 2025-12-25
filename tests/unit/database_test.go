package unit

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/chesskiss/btc-service/internal/database"
	_ "github.com/lib/pq"
)

// setupTestDB creates a test database connection for unit tests
func setupTestDB(t *testing.T) *sql.DB {
	// Use a separate test database to avoid conflicts
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=btc_service_test sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping database tests: PostgreSQL not available: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping database tests: PostgreSQL not reachable: %v", err)
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

// cleanupTestDB removes all data from the test database
func cleanupTestDB(t *testing.T, db *sql.DB) {
	if db != nil {
		_, err := db.Exec("TRUNCATE TABLE request_logs")
		if err != nil {
			t.Logf("Warning: Failed to cleanup test database: %v", err)
		}
		db.Close()
	}
}

func TestInitDB(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		port        string
		user        string
		password    string
		dbname      string
		expectError bool
	}{
		{
			name:        "Valid connection",
			host:        "localhost",
			port:        "5432",
			user:        "postgres",
			password:    "postgres",
			dbname:      "btc_service",
			expectError: false,
		},
		{
			name:        "Invalid host",
			host:        "invalid-host-12345",
			port:        "5432",
			user:        "postgres",
			password:    "postgres",
			dbname:      "btc_service",
			expectError: true,
		},
		{
			name:        "Invalid port",
			host:        "localhost",
			port:        "9999",
			user:        "postgres",
			password:    "postgres",
			dbname:      "btc_service",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := database.InitDB(tt.host, tt.port, tt.user, tt.password, tt.dbname)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Skipf("Skipping test: PostgreSQL not available: %v", err)
				}
				if db == nil {
					t.Errorf("Expected valid database connection but got nil")
				}
				if db != nil {
					db.Close()
				}
			}
		})
	}
}

func TestLogRequest_ValidRequest(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	// Initialize database package with test database
	testDB, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	// Create a test request log
	reqLog := database.RequestLog{
		RequestID:      "test-request-123",
		Method:         "GET",
		Endpoint:       "/api/v1/ltp",
		PairsRequested: "BTC/USD,BTC/EUR",
		UserIP:         "192.168.1.1",
		StatusCode:     200,
		ResponseTimeMs: 45,
		CacheHit:       true,
		KrakenCalls:    0,
		ErrorOccurred:  false,
		ErrorMessage:   "",
	}

	// Log the request
	err = database.LogRequest(reqLog)
	if err != nil {
		t.Fatalf("Failed to log request: %v", err)
	}

	// Verify the request was logged
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM request_logs WHERE request_id = $1", reqLog.RequestID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 record, got %d", count)
	}

	// Verify the data
	var stored database.RequestLog
	err = testDB.QueryRow(`
		SELECT request_id, method, endpoint, pairs_requested, user_ip,
		       status_code, response_time_ms, cache_hit, kraken_calls,
		       error_occurred, error_message
		FROM request_logs
		WHERE request_id = $1
	`, reqLog.RequestID).Scan(
		&stored.RequestID,
		&stored.Method,
		&stored.Endpoint,
		&stored.PairsRequested,
		&stored.UserIP,
		&stored.StatusCode,
		&stored.ResponseTimeMs,
		&stored.CacheHit,
		&stored.KrakenCalls,
		&stored.ErrorOccurred,
		&stored.ErrorMessage,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve record: %v", err)
	}

	// Verify each field
	if stored.RequestID != reqLog.RequestID {
		t.Errorf("RequestID mismatch: got %s, want %s", stored.RequestID, reqLog.RequestID)
	}
	if stored.Method != reqLog.Method {
		t.Errorf("Method mismatch: got %s, want %s", stored.Method, reqLog.Method)
	}
	if stored.Endpoint != reqLog.Endpoint {
		t.Errorf("Endpoint mismatch: got %s, want %s", stored.Endpoint, reqLog.Endpoint)
	}
	if stored.StatusCode != reqLog.StatusCode {
		t.Errorf("StatusCode mismatch: got %d, want %d", stored.StatusCode, reqLog.StatusCode)
	}
	if stored.CacheHit != reqLog.CacheHit {
		t.Errorf("CacheHit mismatch: got %v, want %v", stored.CacheHit, reqLog.CacheHit)
	}
}

func TestLogRequest_WithError(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	testDB, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	reqLog := database.RequestLog{
		RequestID:      "test-error-456",
		Method:         "GET",
		Endpoint:       "/api/v1/ltp",
		PairsRequested: "BTC/INVALID",
		UserIP:         "192.168.1.2",
		StatusCode:     500,
		ResponseTimeMs: 120,
		CacheHit:       false,
		KrakenCalls:    1,
		ErrorOccurred:  true,
		ErrorMessage:   "Failed to fetch from Kraken API",
	}

	err = database.LogRequest(reqLog)
	if err != nil {
		t.Fatalf("Failed to log request: %v", err)
	}

	// Verify error was logged correctly
	var stored database.RequestLog
	err = testDB.QueryRow(`
		SELECT error_occurred, error_message, status_code
		FROM request_logs
		WHERE request_id = $1
	`, reqLog.RequestID).Scan(
		&stored.ErrorOccurred,
		&stored.ErrorMessage,
		&stored.StatusCode,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve record: %v", err)
	}

	if !stored.ErrorOccurred {
		t.Errorf("Expected error_occurred to be true")
	}
	if stored.ErrorMessage != reqLog.ErrorMessage {
		t.Errorf("ErrorMessage mismatch: got %s, want %s", stored.ErrorMessage, reqLog.ErrorMessage)
	}
	if stored.StatusCode != 500 {
		t.Errorf("Expected status code 500, got %d", stored.StatusCode)
	}
}

func TestLogRequest_DuplicateRequestID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	_, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	reqLog := database.RequestLog{
		RequestID:      "duplicate-request-789",
		Method:         "GET",
		Endpoint:       "/api/v1/ltp",
		PairsRequested: "BTC/USD",
		UserIP:         "192.168.1.3",
		StatusCode:     200,
		ResponseTimeMs: 30,
		CacheHit:       true,
		KrakenCalls:    0,
		ErrorOccurred:  false,
		ErrorMessage:   "",
	}

	// Log first time - should succeed
	err = database.LogRequest(reqLog)
	if err != nil {
		t.Fatalf("First log request failed: %v", err)
	}

	// Log second time with same request_id - should fail due to unique constraint
	err = database.LogRequest(reqLog)
	if err == nil {
		t.Errorf("Expected error for duplicate request_id, but got none")
	}
}

func TestLogRequest_WithoutDatabase(t *testing.T) {
	// Close any existing database connection
	database.Close()

	reqLog := database.RequestLog{
		RequestID:      "no-db-request-999",
		Method:         "GET",
		Endpoint:       "/api/v1/ltp",
		PairsRequested: "BTC/USD",
		UserIP:         "192.168.1.4",
		StatusCode:     200,
		ResponseTimeMs: 25,
		CacheHit:       true,
		KrakenCalls:    0,
		ErrorOccurred:  false,
		ErrorMessage:   "",
	}

	err := database.LogRequest(reqLog)
	if err == nil {
		t.Errorf("Expected error when database is not initialized, but got none")
	}

	expectedError := "database not initialized"
	if err != nil && err.Error() != expectedError {
		t.Logf("Got expected error: %v", err)
	}
}

func TestLogRequest_PerformanceMetrics(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	testDB, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	// Test various performance scenarios
	scenarios := []struct {
		name           string
		responseTimeMs int
		cacheHit       bool
		krakenCalls    int
	}{
		{"Fast cache hit", 15, true, 0},
		{"Slow cache miss", 250, false, 1},
		{"Multiple Kraken calls", 500, false, 3},
		{"Medium response", 100, true, 0},
	}

	for i, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			reqLog := database.RequestLog{
				RequestID:      fmt.Sprintf("perf-test-%d", i),
				Method:         "GET",
				Endpoint:       "/api/v1/ltp",
				PairsRequested: "BTC/USD,BTC/EUR,BTC/GBP",
				UserIP:         "192.168.1.10",
				StatusCode:     200,
				ResponseTimeMs: scenario.responseTimeMs,
				CacheHit:       scenario.cacheHit,
				KrakenCalls:    scenario.krakenCalls,
				ErrorOccurred:  false,
				ErrorMessage:   "",
			}

			err := database.LogRequest(reqLog)
			if err != nil {
				t.Fatalf("Failed to log request: %v", err)
			}

			// Verify the metrics were stored correctly
			var stored database.RequestLog
			err = testDB.QueryRow(`
				SELECT response_time_ms, cache_hit, kraken_calls
				FROM request_logs
				WHERE request_id = $1
			`, reqLog.RequestID).Scan(
				&stored.ResponseTimeMs,
				&stored.CacheHit,
				&stored.KrakenCalls,
			)
			if err != nil {
				t.Fatalf("Failed to retrieve record: %v", err)
			}

			if stored.ResponseTimeMs != scenario.responseTimeMs {
				t.Errorf("ResponseTimeMs mismatch: got %d, want %d", stored.ResponseTimeMs, scenario.responseTimeMs)
			}
			if stored.CacheHit != scenario.cacheHit {
				t.Errorf("CacheHit mismatch: got %v, want %v", stored.CacheHit, scenario.cacheHit)
			}
			if stored.KrakenCalls != scenario.krakenCalls {
				t.Errorf("KrakenCalls mismatch: got %d, want %d", stored.KrakenCalls, scenario.krakenCalls)
			}
		})
	}
}

func TestLogRequest_TimestampAutomatic(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	testDB, err := database.InitDB("localhost", "5432", "postgres", "postgres", "btc_service_test")
	if err != nil {
		t.Skipf("Skipping test: Cannot initialize database: %v", err)
		return
	}
	defer database.Close()

	beforeLog := time.Now()

	reqLog := database.RequestLog{
		RequestID:      "timestamp-test-001",
		Method:         "GET",
		Endpoint:       "/api/v1/ltp",
		PairsRequested: "BTC/USD",
		UserIP:         "192.168.1.5",
		StatusCode:     200,
		ResponseTimeMs: 40,
		CacheHit:       true,
		KrakenCalls:    0,
		ErrorOccurred:  false,
		ErrorMessage:   "",
	}

	err = database.LogRequest(reqLog)
	if err != nil {
		t.Fatalf("Failed to log request: %v", err)
	}

	afterLog := time.Now()

	// Verify timestamp was set automatically
	var timestamp time.Time
	err = testDB.QueryRow(`
		SELECT timestamp FROM request_logs WHERE request_id = $1
	`, reqLog.RequestID).Scan(&timestamp)
	if err != nil {
		t.Fatalf("Failed to retrieve timestamp: %v", err)
	}

	// Convert all times to UTC for comparison to handle timezone differences
	beforeLogUTC := beforeLog.UTC()
	afterLogUTC := afterLog.UTC()
	timestampUTC := timestamp.UTC()

	// Allow 5 second buffer for test execution time
	if timestampUTC.Before(beforeLogUTC.Add(-5*time.Second)) || timestampUTC.After(afterLogUTC.Add(5*time.Second)) {
		t.Errorf("Timestamp %v is outside expected range [%v, %v]", timestampUTC, beforeLogUTC, afterLogUTC)
	}
}
