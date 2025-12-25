package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/redis/go-redis/v9"
)

// HealthHandler returns basic health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// ReadinessHandler checks database and cache connectivity
func ReadinessHandler(db *sql.DB, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ctx := context.Background()

		// Check database connection
		if db != nil {
			if err := db.Ping(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{
					"status": "not ready",
					"error":  "database unavailable",
				})
				return
			}
		}

		// Check Redis connection
		if redisClient != nil {
			if err := redisClient.Ping(ctx).Err(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{
					"status": "not ready",
					"error":  "cache unavailable",
				})
				return
			}
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status": "ready",
		})
	}
}
