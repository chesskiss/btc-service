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

### Run 
```bash
docker run -p 8080:8080 btc-service

curl http://localhost:8080/api/v1/ltp
```

Note- The service runs on port 8080 by default. You can customize this with the `PORT` environment variable:
```bash
# Run on a different port
docker run -p 9000:9000 -e PORT=9000 btc-service
```


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

### Response Format

```json
{
  "ltp": [
    {
      "pair": "BTC/USD",
      "amount": 52000.12
    },
    {
      "pair": "BTC/EUR",
      "amount": 50000.12
    }
  ]
}
```


## Testing

Run all tests:
```bash
go test ./...
```

Run with coverage:
```bash
go test -cover ./...
```

Run integration tests:
```bash
go test -v ./tests/integration/...
```


