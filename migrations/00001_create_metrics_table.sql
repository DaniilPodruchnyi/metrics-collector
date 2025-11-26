-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS metrics (
    id VARCHAR(255) PRIMARY KEY,
    type VARCHAR(10) NOT NULL CHECK (type IN ('gauge', 'counter')),
    delta BIGINT,
    value DOUBLE PRECISION,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT check_metric_value CHECK (
        (type = 'counter' AND delta IS NOT NULL AND value IS NULL) OR
        (type = 'gauge' AND value IS NOT NULL AND delta IS NULL)
    )
);

CREATE INDEX idx_metrics_type ON metrics(type);
CREATE INDEX idx_metrics_updated_at ON metrics(updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_metrics_updated_at;
DROP INDEX IF EXISTS idx_metrics_type;
DROP TABLE IF EXISTS metrics;
-- +goose StatementEnd
