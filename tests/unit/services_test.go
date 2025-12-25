package unit

import (
	"context"
	"testing"

	"github.com/chesskiss/btc-service/services"
)

func TestGetPricesDefault(t *testing.T) {
	result := services.GetPrices(context.Background(), "")

	if result.Prices == nil {
		t.Error("expected non-nil slice")
	}

	// Default should request 3 currencies (USD, EUR, CHF)
	if result.KrakenCalls != 3 {
		t.Errorf("expected 3 Kraken calls for default, got %d", result.KrakenCalls)
	}
}

func TestGetPricesWithParam(t *testing.T) {
	result := services.GetPrices(context.Background(), "BTC/USD")

	if result.Prices == nil {
		t.Error("expected non-nil slice")
	}

	// Single pair should result in 1 Kraken call
	if result.KrakenCalls != 1 {
		t.Errorf("expected 1 Kraken call, got %d", result.KrakenCalls)
	}
}

func TestPairPriceStruct(t *testing.T) {
	p := services.PairPrice{
		Pair:   "BTC/USD",
		Amount: 50000.0,
	}

	if p.Pair != "BTC/USD" {
		t.Errorf("got %s, want BTC/USD", p.Pair)
	}
	if p.Amount != 50000.0 {
		t.Errorf("got %f, want 50000.0", p.Amount)
	}
}
