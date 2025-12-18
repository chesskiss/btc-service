package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chesskiss/btc-service/handlers"
)

func TestLTPHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/ltp", nil)
	w := httptest.NewRecorder()

	handlers.LTPHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q, want %q", ct, "application/json")
	}
}

func TestLTPHandlerWithPairs(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD", nil)
	w := httptest.NewRecorder()

	handlers.LTPHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}
