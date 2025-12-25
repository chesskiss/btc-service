CREATE TABLE request_logs (
    id SERIAL PRIMARY KEY,
    request_id VARCHAR(36) UNIQUE,
    timestamp TIMESTAMP DEFAULT NOW(),

    -- Request
    method VARCHAR(10),
    endpoint VARCHAR(100),
    pairs_requested TEXT,
    user_ip VARCHAR(45),

    -- Response
    status_code INT,
    response_time_ms INT,

    -- Performance
    cache_hit BOOLEAN,
    kraken_calls INT,

    -- Errors
    error_occurred BOOLEAN,
    error_message TEXT
);

CREATE INDEX idx_timestamp ON request_logs(timestamp);
CREATE INDEX idx_status ON request_logs(status_code);
