package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

var db *sql.DB

type RequestLog struct {
	RequestID      string
	Method         string
	Endpoint       string
	PairsRequested string
	UserIP         string
	StatusCode     int
	ResponseTimeMs int
	CacheHit       bool
	KrakenCalls    int
	ErrorOccurred  bool
	ErrorMessage   string
}

// InitDB initializes the PostgreSQL database connection
func InitDB(host, port, user, password, dbname string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		log.Printf("Warning: Failed to connect to PostgreSQL: %v", err)
		log.Println("Continuing without request logging...")
		return nil, err
	}

	log.Println("PostgreSQL connected successfully")
	return db, nil
}

// LogRequest inserts a request log entry into the database
func LogRequest(reqLog RequestLog) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
		INSERT INTO request_logs (
			request_id, method, endpoint, pairs_requested, user_ip,
			status_code, response_time_ms, cache_hit, kraken_calls,
			error_occurred, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := db.Exec(query,
		reqLog.RequestID,
		reqLog.Method,
		reqLog.Endpoint,
		reqLog.PairsRequested,
		reqLog.UserIP,
		reqLog.StatusCode,
		reqLog.ResponseTimeMs,
		reqLog.CacheHit,
		reqLog.KrakenCalls,
		reqLog.ErrorOccurred,
		reqLog.ErrorMessage,
	)

	if err != nil {
		log.Printf("Failed to log request to database: %v", err)
		return err
	}

	return nil
}

// Close closes the database connection
func Close() {
	if db != nil {
		db.Close()
	}
}
