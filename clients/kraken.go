package clients

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

// For encoding/decoding Kraken JSON
type KrakenResponse struct {
    Error  []string              `json:"error"`
    Result map[string]KrakenPair `json:"result"`
}

type KrakenPair struct {
    C []string `json:"c"` // last trade closed: [price, lot volume]
}

// GetBTCPrice fetches the BTC price in the given currency from Kraken API
func GetBTCPrice(currency string) (float64, error) {
    pair := fmt.Sprintf("XBT%s", currency)
    url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pair)

    resp, err := http.Get(url)
    if err != nil {
        return 0, fmt.Errorf("failed to make request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
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

    for _, pairData := range krakenResp.Result {
        if len(pairData.C) > 0 {
            var price float64
            if _, err := fmt.Sscanf(pairData.C[0], "%f", &price); err != nil {
                return 0, fmt.Errorf("failed to parse price: %w", err)
            }
            return price, nil
        }
    }

    return 0, fmt.Errorf("no price data found")
}
