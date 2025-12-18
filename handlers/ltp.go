package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/chesskiss/btc-service/services"
)

func LTPHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    pairsParam := r.URL.Query().Get("pairs")
    prices := services.GetPrices(pairsParam)

    json.NewEncoder(w).Encode(
        services.LTPResponse{LTP: prices},
    )
}
