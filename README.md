# Bitcoin LTP Service

A Go microservice that provides Last Traded Price data for Bitcoin across multiple currency pairs. Price data is fetched from the Kraken public API.

## Prerequisites
- Docker 20.10+


## Setup and usage

```bash
# Clone repository
git clone https://github.com/chesskiss/btc-service.git
cd btc-service

# Build & run
docker-compose up -d 
```

Note- The service runs on port 8080 by default. You can customize this with the `PORT` environment variable:
```bash
docker run --name btc-service -p 9000:9000 -e PORT=<YOUR PREFFFFERED PORT> btc-service
```


### Stop process

```bash
docker-compose down
```


### To send an http request:

**Query Parameters:**

- `pairs` (optional): Comma-separated list of currency pairs (e.g., `BTC/USD,BTC/EUR`)
- If omitted, returns all supported pairs: BTC/USD, BTC/EUR, BTC/CHF

#### Examples

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


## Observability

### Metrics (Prometheus)
View metrics:
```bash
curl http://localhost:8080/metrics
```

Key metrics:
- `http_requests_total` - Total HTTP requests by method, endpoint, status
- `http_request_duration_seconds` - Request duration histogram
- `cache_hits_total` / `cache_misses_total` - Cache performance
- `kraken_api_calls_total` / `kraken_api_errors_total` - External API metrics


Or with **Graphana** visualization, go to:
- URL: http://localhost:3000
- Login: admin / admin
- Dashboard: "BTC Service Overview"

### Distributed Tracing (OpenTelemetry) 
Use it to track request flow and timing across all components:
- Request timelines with nested spans
- Cache vs API call timing comparisons
- Error tracking and debugging
- Performance bottleneck identification

View traces in logs:
```bash
docker-compose logs -f btc-service | grep -B 2 -A 100 '"Name":'

# Or view specific trace attributes
docker-compose logs -f btc-service | grep -A 30 "SpanContext"
```

Or, with Jaeger UI visualization:
- URL: http://localhost:16686
- Find traces by operation name:
  - `handle_ltp_request` - Full HTTP request handling
  - `get_prices` - Price fetching logic
  - `get_btc_price` - Individual currency price fetch
  - `check_cache` - Redis cache operations
  - `fetch_from_kraken` - External API calls


### Health Checks
```bash
# Liveness probe
curl http://localhost:8080/health

# Readiness probe (checks DB and Redis)
curl http://localhost:8080/ready
```

### Structured Logs
Logs are output in JSON format with structured fields:
```bash
docker-compose logs -f btc-service
```

Log fields: `timestamp`, `level`, `message`, `request_id`, `pair`, `error`, `duration_ms`

### Database Analytics
Access PostgreSQL for request analytics:
```bash
docker exec -it btc-postgres psql -U postgres -d btc_service
```

Example queries:
```sql
-- View recent requests
SELECT * FROM request_logs ORDER BY timestamp DESC LIMIT 10;

-- Cache hit rate
SELECT
  COUNT(*) FILTER (WHERE cache_hit = true) * 100.0 / COUNT(*) as cache_hit_rate
FROM request_logs;

-- Average response time
SELECT AVG(response_time_ms) as avg_response_time FROM request_logs;
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




## Response Format

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

