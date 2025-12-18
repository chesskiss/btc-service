# Bitcoin LTP Service

A Go microservice that provides Last Traded Price data for Bitcoin across multiple currency pairs. Price data is fetched from the Kraken public API.

## Prerequisites
- Docker 20.10+


## Setup

```bash
# Clone repository
git clone https://github.com/yourusername/btc-service.git
cd btc-service

# Build
docker build -t btc-service .
```

## Usage

### Run process
```bash
docker run --name btc-service -p 8080:8080 btc-service

```

Note- The service runs on port 8080 by default. You can customize this with the `PORT` environment variable:
```bash
docker run --name btc-service -p 9000:9000 -e PORT=<YOUR PREFFFFERED PORT> btc-service
```

### Stop process

From a new terminal run 
```bash
docker rm -f btc-service
```
Or simply ctrl+C

### To send an http request:

**Query Parameters:**

- `pairs` (optional): Comma-separated list of currency pairs (e.g., `BTC/USD,BTC/EUR`)
- If omitted, returns all supported pairs: BTC/USD, BTC/EUR, BTC/CHF

### Examples

Get all pairs:
```bash
curl http://localhost:8080/api/v1/ltp
```

Get specific pair:
```bash
curl http://localhost:8080/api/v1/ltp?pairs=BTC/USD
```

Get multiple pairs:
```bash
curl "http://localhost:8080/api/v1/ltp?pairs=BTC/USD,BTC/EUR"
```


## Testing

Run all tests:
```bash
go test ./tests/...
```

Run with coverage:
```bash
go test -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```




### Response Format

Given curl "http://localhost:8080/api/v1/ltp?pairs=BTC/<currency 1>,BTC/<currency 2>..."
The returned response:
```json
{
  "ltp": [
    {
      "pair": "BTC/<currency 1>",
      "amount": 0000.00
    },
    {
      "pair": "BTC/<currency 2>",
      "amount": 0000.00
    }
    ...
  ]
}
```
