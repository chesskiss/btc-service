package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"

    "github.com/gorilla/mux" 
)


// LTPResponse represents the response for last trade price
type LTPResponse struct {
    LTP []PairPrice `json:"ltp"`
}

// PairPrice represents a single pair's price
type PairPrice struct {
    Pair   string  `json:"pair"`
    Amount float64 `json:"amount"`
}

// For encoding/decoding Kraken JSON
type KrakenResponse struct {
    Error  []string               `json:"error"`
    Result map[string]KrakenPair  `json:"result"`
}

type KrakenPair struct {
    C []string `json:"c"` // Based on api tutorial, "c" = last trade closed: [price, lot volume]
}


// getBTCPrice fetches the BTC price in the given currency from Kraken API
func getBTCPrice(currency string) (float64, error) {
    // Construct the pair for Kraken 
    pair := fmt.Sprintf("XBT%s", currency)

    // Kraken API endpoint 
    url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pair)

    resp, err := http.Get(url)  // Make the HTTP GET request
    if err != nil {
        return 0, fmt.Errorf("failed to make request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body) 	// Read the response body
    if err != nil {
        return 0, fmt.Errorf("failed to read response: %w", err)
    }

    
    var krakenResp KrakenResponse 
    if err := json.Unmarshal(body, &krakenResp); err != nil {
        return 0, fmt.Errorf("failed to parse response: %w", err)
    }
	if len(krakenResp.Error) > 0 {
        return 0, fmt.Errorf("kraken API error: %v", krakenResp.Error)
    }

    // Based on api tutorial, "c" = last trade closed
    for _, pairData := range krakenResp.Result {
        if len(pairData.C) > 0 {
            var price float64
            _, err := fmt.Sscanf(pairData.C[0], "%f", &price)
            if err != nil {
                return 0, fmt.Errorf("failed to parse price: %w", err)
            }
            return price, nil
        }
    }

    return 0, fmt.Errorf("no price data found")
}


// extractCurrency extracts the currency from a pair (e.g., "BTC/USD" -> "USD")
func extractCurrency(pair string) string {
    for i, char := range pair {
        if char == '/' {
            if i+1 < len(pair) {
                return pair[i+1:]
            }
            return ""
        }
    }
    return ""
}


// splitPairs splits the pairs parameter by comma
func splitPairs(pairsParam string) []string {
    // Split by comma
    var splitResult []string
    var current string
    for _, char := range pairsParam {
        if char == ',' {
            splitResult = append(splitResult, current)
            current = ""
        } else {
            current += string(char)
        }
    }
    if current != "" {
        splitResult = append(splitResult, current)
    }

    // Trim spaces from each pair
    var pairs []string
    for _, pair := range splitResult {
        // Trim leading and trailing spaces
        start := 0
        end := len(pair)

        for start < end && pair[start] == ' ' {
            start++
        }
        for end > start && pair[end-1] == ' ' {
            end--
        }

        trimmed := pair[start:end]
        if trimmed != "" {
            pairs = append(pairs, trimmed)
        }
    }
    return pairs
}



// ltpHandler handles requests to /api/v1/ltp
func ltpHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    // Get the pairs query parameter
    pairsParam := r.URL.Query().Get("pairs")

	// Input analysis
    var currencies []string
    if pairsParam == "" {
        currencies = []string{"USD", "EUR", "CHF"} // Default: return all supported pairs
    } else {
        pairsList := splitPairs(pairsParam)
        for _, pair := range pairsList {
            // Extract currency from pair (e.g., "BTC/USD" -> "USD")
            currency := extractCurrency(pair)
            if currency != "" {
                currencies = append(currencies, currency)
            }
        }
    }

    // Fetch prices for all requested currencies
    var prices []PairPrice
    for _, currency := range currencies {
        price, err := getBTCPrice(currency)
        if err != nil {
            log.Printf("Error fetching BTC/%s: %v\n", currency, err)
            continue
        }

        prices = append(prices, PairPrice{
            Pair:   fmt.Sprintf("BTC/%s", currency),
            Amount: price,
        })
    }

    response := LTPResponse{LTP: prices}
    json.NewEncoder(w).Encode(response)
}



func main() {
    r := mux.NewRouter()
    r.HandleFunc("/api/v1/ltp", ltpHandler).Methods("GET")

    log.Println("Server on :8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}