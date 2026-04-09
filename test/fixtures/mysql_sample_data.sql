-- Sample data for testing dbridge with MySQL

-- Use mysql_native_password so connections work without TLS
ALTER USER 'dbridge'@'%' IDENTIFIED WITH mysql_native_password BY 'dbridge';

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    active BOOLEAN DEFAULT true,
    age INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    product VARCHAR(100) NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Insert sample users
INSERT IGNORE INTO users (email, name, active, age) VALUES
    ('alice@example.com', 'Alice Johnson', true, 28),
    ('bob@example.com', 'Bob Smith', true, 34),
    ('charlie@example.com', 'Charlie Brown', false, 42),
    ('diana@example.com', 'Diana Prince', true, 31),
    ('eve@example.com', 'Eve Wilson', true, 26),
    ('frank@example.com', 'Frank Miller', false, 45),
    ('grace@example.com', 'Grace Lee', true, 29),
    ('henry@example.com', 'Henry Ford', true, 38),
    ('iris@example.com', 'Iris Chen', true, 33),
    ('jack@example.com', 'Jack Ryan', false, 40);

-- Insert sample orders
INSERT IGNORE INTO orders (user_id, product, amount, status) VALUES
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

-- Table with >10000 rows used by truncation integration tests.
-- MySQL default cte_max_recursion_depth=1000 is too low; raise it for this session.
SET cte_max_recursion_depth = 20000;
CREATE TABLE IF NOT EXISTS large_table (id INT);
INSERT INTO large_table (id)
WITH RECURSIVE n(i) AS (
    SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 10001
)
SELECT i FROM n;

-- Create a view
CREATE OR REPLACE VIEW user_summary AS
SELECT
    u.id,
    u.name,
    u.email,
    COUNT(o.id) AS order_count,
    COALESCE(SUM(o.amount), 0) AS total_spent
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name, u.email;
