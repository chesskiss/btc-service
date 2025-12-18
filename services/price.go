package services

import (
    "fmt"
    "log"

    "github.com/chesskiss/btc-service/clients"
)

type PairPrice struct {
    Pair   string  `json:"pair"`
    Amount float64 `json:"amount"`
}

type LTPResponse struct {
    LTP []PairPrice `json:"ltp"`
}

func GetPrices(pairsParam string) []PairPrice {
    currencies := resolveCurrencies(pairsParam)

    var prices []PairPrice
    for _, currency := range currencies {
        price, err := clients.GetBTCPrice(currency)
        if err != nil {
            log.Printf("Error fetching BTC/%s: %v\n", currency, err)
            continue
        }

        prices = append(prices, PairPrice{
            Pair:   fmt.Sprintf("BTC/%s", currency),
            Amount: price,
        })
    }

    return prices
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
