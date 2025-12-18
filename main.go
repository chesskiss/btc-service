package main

import (
    "log"
    "net/http"

    "github.com/gorilla/mux"

    "github.com/chesskiss/btc-service/handlers"
)

func main() {
    r := mux.NewRouter()
    r.HandleFunc("/api/v1/ltp", handlers.LTPHandler).Methods("GET")

    log.Println("Server on :8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}
