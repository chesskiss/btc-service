package services

import (
    "context"
    "fmt"
    "log"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"

    "github.com/chesskiss/btc-service/clients"
)

type PairPrice struct {
    Pair   string  `json:"pair"`
    Amount float64 `json:"amount"`
}

type LTPResponse struct {
    LTP []PairPrice `json:"ltp"`
}

type PriceResult struct {
    Prices       []PairPrice
    ErrorsCount  int
    KrakenCalls  int
    ErrorMessage string
}

func GetPrices(ctx context.Context, pairsParam string) PriceResult {
    tracer := otel.Tracer("btc-service")
    ctx, span := tracer.Start(ctx, "get_prices")
    defer span.End()

    currencies := resolveCurrencies(pairsParam)

    span.SetAttributes(
        attribute.StringSlice("currencies", currencies),
        attribute.Int("currency_count", len(currencies)),
    )

    var prices []PairPrice
    var errorsCount int
    var lastError string

    for _, currency := range currencies {
        price, err := clients.GetBTCPrice(ctx, currency)
        if err != nil {
            log.Printf("Error fetching BTC/%s: %v\n", currency, err)
            errorsCount++
            lastError = fmt.Sprintf("BTC/%s: %v", currency, err)
            continue
        }

        prices = append(prices, PairPrice{
            Pair:   fmt.Sprintf("BTC/%s", currency),
            Amount: price,
        })
    }

    span.SetAttributes(
        attribute.Int("prices_fetched", len(prices)),
        attribute.Int("errors_count", errorsCount),
    )

    return PriceResult{
        Prices:       prices,
        ErrorsCount:  errorsCount,
        KrakenCalls:  len(currencies), // Each currency requires one Kraken API call
        ErrorMessage: lastError,
    }
}

func resolveCurrencies(pairsParam string) []string {
    if pairsParam == "" {
        return []string{"USD", "EUR", "CHF"}
    }

    pairs := splitPairs(pairsParam)
    var currencies []string
    for _, pair := range pairs {
        if currency := extractCurrency(pair); currency != "" {
            currencies = append(currencies, currency)
        }
    }
    return currencies
}

func extractCurrency(pair string) string {
    for i, char := range pair {
        if char == '/' && i+1 < len(pair) {
            return pair[i+1:]
        }
    }
    return ""
}

func splitPairs(pairsParam string) []string {
    var result []string
    var current string

    for _, char := range pairsParam {
        if char == ',' {
            result = append(result, current)
            current = ""
        } else {
            current += string(char)
        }
    }
    if current != "" {
        result = append(result, current)
    }

    var pairs []string
    for _, pair := range result {
        trimmed := trimSpaces(pair)
        if trimmed != "" {
            pairs = append(pairs, trimmed)
        }
    }

    return pairs
}

func trimSpaces(s string) string {
    start, end := 0, len(s)
    for start < end && s[start] == ' ' {
        start++
    }
    for end > start && s[end-1] == ' ' {
        end--
    }
    return s[start:end]
}
