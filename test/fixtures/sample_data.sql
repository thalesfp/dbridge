-- Sample data for testing dbbridge

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    active BOOLEAN DEFAULT true,
    age INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    product VARCHAR(100) NOT NULL,
    amount NUMERIC(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create analytics schema
CREATE SCHEMA IF NOT EXISTS analytics;

-- Create analytics table
CREATE TABLE IF NOT EXISTS analytics.events (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    user_id INTEGER,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample users
INSERT INTO users (email, name, active, age) VALUES
    ('alice@example.com', 'Alice Johnson', true, 28),
    ('bob@example.com', 'Bob Smith', true, 34),
    ('charlie@example.com', 'Charlie Brown', false, 42),
    ('diana@example.com', 'Diana Prince', true, 31),
    ('eve@example.com', 'Eve Wilson', true, 26),
    ('frank@example.com', 'Frank Miller', false, 45),
    ('grace@example.com', 'Grace Lee', true, 29),
    ('henry@example.com', 'Henry Ford', true, 38),
    ('iris@example.com', 'Iris Chen', true, 33),
    ('jack@example.com', 'Jack Ryan', false, 40)
ON CONFLICT (email) DO NOTHING;

-- Insert sample orders
INSERT INTO orders (user_id, product, amount, status) VALUES
    (1, 'Laptop', 999.99, 'completed'),
    (1, 'Mouse', 29.99, 'completed'),
    (2, 'Keyboard', 79.99, 'pending'),
    (3, 'Monitor', 299.99, 'completed'),
    (4, 'Headphones', 149.99, 'shipped'),
    (5, 'Webcam', 89.99, 'pending'),
    (2, 'USB Cable', 12.99, 'completed'),
    (7, 'Laptop Stand', 49.99, 'shipped'),
    (8, 'Desk Lamp', 34.99, 'completed'),
    (1, 'Phone Case', 19.99, 'pending');

-- Insert sample analytics events
INSERT INTO analytics.events (event_type, user_id, metadata) VALUES
    ('login', 1, '{"ip": "192.168.1.1", "browser": "Chrome"}'),
    ('page_view', 1, '{"page": "/dashboard", "duration": 45}'),
    ('login', 2, '{"ip": "192.168.1.2", "browser": "Firefox"}'),
    ('purchase', 1, '{"product_id": 101, "amount": 999.99}'),
    ('logout', 1, '{"session_duration": 1800}'),
    ('login', 4, '{"ip": "192.168.1.4", "browser": "Safari"}'),
    ('page_view', 4, '{"page": "/products", "duration": 120}'),
    ('purchase', 4, '{"product_id": 102, "amount": 149.99}');

-- Table with >10000 rows used by truncation integration tests.
CREATE TABLE IF NOT EXISTS large_table AS
SELECT i AS id FROM generate_series(1, 10001) AS t(i);

-- Create a view
CREATE OR REPLACE VIEW user_summary AS
SELECT
    u.id,
    u.name,
    u.email,
    COUNT(o.id) as order_count,
    COALESCE(SUM(o.amount), 0) as total_spent
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name, u.email;
