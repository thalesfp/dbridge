-- Rich schema fixture exercising all 16 PostgreSQL types supported by dbridge
-- Types: int2, int4, int8, float4, float8, numeric, text, varchar, bool,
--        date, time, timestamp, timestamptz, json, jsonb, uuid

-- =============================================================================
-- Public schema: type_showcase
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE type_showcase (
    id              SERIAL PRIMARY KEY,
    col_int2        SMALLINT,
    col_int4        INTEGER NOT NULL,
    col_int8        BIGINT,
    col_float4      REAL,
    col_float8      DOUBLE PRECISION,
    col_numeric     NUMERIC(12, 4),
    col_text        TEXT,
    col_varchar     VARCHAR(255),
    col_bool        BOOLEAN DEFAULT true,
    col_date        DATE,
    col_time        TIME,
    col_timestamp   TIMESTAMP,
    col_timestamptz TIMESTAMPTZ DEFAULT NOW(),
    col_json        JSON,
    col_jsonb       JSONB,
    col_uuid        UUID DEFAULT uuid_generate_v4(),
    CONSTRAINT chk_int4_positive CHECK (col_int4 > 0)
);

-- Indexes
CREATE INDEX idx_type_showcase_int4 ON type_showcase USING btree (col_int4);
CREATE INDEX idx_type_showcase_jsonb ON type_showcase USING gin (col_jsonb);
CREATE UNIQUE INDEX idx_type_showcase_uuid ON type_showcase (col_uuid);

-- Normal values
INSERT INTO type_showcase (
    col_int2, col_int4, col_int8, col_float4, col_float8, col_numeric,
    col_text, col_varchar, col_bool, col_date, col_time,
    col_timestamp, col_timestamptz, col_json, col_jsonb, col_uuid
) VALUES
(1, 100, 1000000000, 3.14, 2.718281828459045, 99999.1234,
 'Hello, world!', 'short text', true, '2024-01-15', '14:30:00',
 '2024-01-15 14:30:00', '2024-01-15 14:30:00+00', '{"key": "value"}', '{"tags": ["a", "b"]}',
 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'),
(2, 200, 2000000000, -1.5, 0.0, 0.0001,
 'Another row', 'varchar value', false, '2023-06-01', '09:00:00',
 '2023-06-01 09:00:00', '2023-06-01 09:00:00+05:30', '{"nested": {"deep": true}}', '{"count": 42}',
 'b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22');

-- Boundary values
INSERT INTO type_showcase (
    col_int2, col_int4, col_int8, col_float4, col_float8, col_numeric,
    col_text, col_varchar, col_bool, col_date, col_time,
    col_timestamp, col_timestamptz, col_json, col_jsonb, col_uuid
) VALUES
(-32768, 1, -9223372036854775808, 1.175e-38, 2.225e-308, -99999999.9999,
 '', '', true, '0001-01-01', '00:00:00',
 '0001-01-01 00:00:00', '0001-01-01 00:00:00+00', '[]', '{}',
 'c2eebc99-9c0b-4ef8-bb6d-6bb9bd380a33'),
(32767, 2, 9223372036854775807, 3.4028235e+38, 1.7976931348623157e+308, 99999999.9999,
 repeat('x', 1000), repeat('y', 255), false, '9999-12-31', '23:59:59',
 '9999-12-31 23:59:59', '9999-12-31 23:59:59+00', '{"max": true}', '{"max": true}',
 'd3eebc99-9c0b-4ef8-bb6d-6bb9bd380a44');

-- NULL values (except col_int4 which is NOT NULL)
INSERT INTO type_showcase (col_int4) VALUES (3);

-- Large text and complex JSON
INSERT INTO type_showcase (
    col_int4, col_text, col_json, col_jsonb
) VALUES
(4, repeat('long text ', 100),
 '{"array": [1, 2, 3], "object": {"nested": {"deeply": "value"}}, "null_field": null}',
 '{"search": "indexable", "numbers": [1, 2, 3], "nested": {"key": "value"}}');

-- =============================================================================
-- Analytics schema
-- =============================================================================

CREATE SCHEMA IF NOT EXISTS analytics;

CREATE TABLE analytics.events (
    id          SERIAL PRIMARY KEY,
    event_name  VARCHAR(100) NOT NULL,
    user_id     INTEGER,
    properties  JSONB DEFAULT '{}',
    occurred_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE analytics.metrics (
    id          SERIAL PRIMARY KEY,
    metric_name VARCHAR(100) NOT NULL,
    value       NUMERIC(15, 4) NOT NULL,
    dimensions  JSONB DEFAULT '{}',
    recorded_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO analytics.events (event_name, user_id, properties) VALUES
('page_view', 1, '{"page": "/home", "referrer": "google.com"}'),
('click', 1, '{"element": "buy_button", "page": "/product/42"}'),
('page_view', 2, '{"page": "/about"}'),
('signup', 3, '{"source": "organic", "plan": "free"}'),
('purchase', 1, '{"product_id": 42, "amount": 29.99, "currency": "USD"}');

INSERT INTO analytics.metrics (metric_name, value, dimensions) VALUES
('page_load_time', 1.234, '{"page": "/home", "region": "us-east"}'),
('api_latency', 0.0567, '{"endpoint": "/api/users", "method": "GET"}'),
('error_rate', 0.0012, '{"service": "auth", "env": "production"}');

-- =============================================================================
-- Reports schema
-- =============================================================================

CREATE SCHEMA IF NOT EXISTS reports;

CREATE TABLE reports.daily_summary (
    id            SERIAL PRIMARY KEY,
    report_date   DATE NOT NULL UNIQUE,
    total_users   INTEGER DEFAULT 0,
    total_events  INTEGER DEFAULT 0,
    total_revenue NUMERIC(12, 2) DEFAULT 0.00,
    metadata      JSONB DEFAULT '{}'
);

INSERT INTO reports.daily_summary (report_date, total_users, total_events, total_revenue, metadata) VALUES
('2024-01-15', 150, 3200, 4599.99, '{"top_page": "/home", "new_signups": 12}'),
('2024-01-16', 165, 3800, 5200.00, '{"top_page": "/products", "new_signups": 18}'),
('2024-01-17', 142, 2900, 3100.50, '{"top_page": "/home", "new_signups": 8}');

-- =============================================================================
-- Views
-- =============================================================================

-- Regular view with JOIN
CREATE VIEW public.event_user_summary AS
SELECT
    e.event_name,
    COUNT(*) AS event_count,
    COUNT(DISTINCT e.user_id) AS unique_users,
    MIN(e.occurred_at) AS first_seen,
    MAX(e.occurred_at) AS last_seen
FROM analytics.events e
GROUP BY e.event_name;

-- View with CTE and window function
CREATE VIEW reports.revenue_trend AS
WITH daily AS (
    SELECT
        report_date,
        total_revenue,
        LAG(total_revenue) OVER (ORDER BY report_date) AS prev_revenue
    FROM reports.daily_summary
)
SELECT
    report_date,
    total_revenue,
    prev_revenue,
    CASE
        WHEN prev_revenue IS NOT NULL AND prev_revenue > 0
        THEN ROUND(((total_revenue - prev_revenue) / prev_revenue) * 100, 2)
        ELSE NULL
    END AS pct_change
FROM daily;
