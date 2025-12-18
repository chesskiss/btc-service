package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"github.com/chesskiss/btc-service/handlers"
	"github.com/chesskiss/btc-service/services"
)

func createTestServer() *httptest.Server {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")
	return httptest.NewServer(r)
}

func TestAPIEndpointDefault(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/ltp")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("got Content-Type %q, want application/json", ct)
	}

	var ltpResp services.LTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&ltpResp); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	if ltpResp.LTP == nil {
		t.Error("expected non-nil LTP slice")
	}
}

func TestAPIEndpointWithPairs(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/ltp?pairs=BTC/USD,BTC/EUR")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var ltpResp services.LTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&ltpResp); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	if ltpResp.LTP == nil {
		t.Error("expected non-nil LTP slice")
	}
}

func TestAPIEndpointInvalidMethod(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	for _, method := range []string{"POST", "PUT", "DELETE"} {
		req, _ := http.NewRequest(method, server.URL+"/api/v1/ltp", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s request failed: %v", method, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Errorf("%s should not be allowed", method)
		}
	}
}
