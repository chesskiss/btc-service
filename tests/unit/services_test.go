package unit

import (
	"testing"

	"github.com/chesskiss/btc-service/services"
)

func TestGetPricesDefault(t *testing.T) {
	prices := services.GetPrices("")

	if prices == nil {
		t.Error("expected non-nil slice")
	}
}

func TestGetPricesWithParam(t *testing.T) {
	prices := services.GetPrices("BTC/USD")

	if prices == nil {
		t.Error("expected non-nil slice")
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
