package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chesskiss/btc-service/handlers"
	"github.com/chesskiss/btc-service/internal/middleware"
	"github.com/gorilla/mux"
)

func TestLTPHandler(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	req := httptest.NewRequest("GET", "/api/v1/ltp", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q, want %q", ct, "application/json")
	}
}

func TestLTPHandlerWithPairs(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	handler := middleware.LoggingMiddleware(r)

	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}
