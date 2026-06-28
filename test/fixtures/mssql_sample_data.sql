-- Sample data for testing dbridge with SQL Server.
-- Loaded via sqlcmd -i; GO separates batches (CREATE VIEW must be its own batch).

IF OBJECT_ID('dbo.user_summary', 'V') IS NOT NULL DROP VIEW dbo.user_summary;
IF OBJECT_ID('dbo.orders', 'U') IS NOT NULL DROP TABLE dbo.orders;
IF OBJECT_ID('dbo.users', 'U') IS NOT NULL DROP TABLE dbo.users;
IF OBJECT_ID('dbo.large_table', 'U') IS NOT NULL DROP TABLE dbo.large_table;
IF OBJECT_ID('dbo._dbridge_ready', 'U') IS NOT NULL DROP TABLE dbo._dbridge_ready;
GO

CREATE TABLE dbo.users (
    id INT IDENTITY(1,1) PRIMARY KEY,
    email NVARCHAR(255) NOT NULL UNIQUE,
    name NVARCHAR(100) NOT NULL,
    active BIT NOT NULL DEFAULT 1,
    age INT,
    created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
GO

CREATE TABLE dbo.orders (
    id INT IDENTITY(1,1) PRIMARY KEY,
    user_id INT,
    product NVARCHAR(100) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    status NVARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT FK_orders_users FOREIGN KEY (user_id) REFERENCES dbo.users(id)
);
GO

INSERT INTO dbo.users (email, name, active, age) VALUES
    ('alice@example.com', 'Alice Johnson', 1, 28),
    ('bob@example.com', 'Bob Smith', 1, 34),
    ('charlie@example.com', 'Charlie Brown', 0, 42),
    ('diana@example.com', 'Diana Prince', 1, 31),
    ('eve@example.com', 'Eve Wilson', 1, 26),
    ('frank@example.com', 'Frank Miller', 0, 45),
    ('grace@example.com', 'Grace Lee', 1, 29),
    ('henry@example.com', 'Henry Ford', 1, 38),
    ('iris@example.com', 'Iris Chen', 1, 33),
    ('jack@example.com', 'Jack Ryan', 0, 40);
GO

INSERT INTO dbo.orders (user_id, product, amount, status) VALUES
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
GO

-- Table with >10000 rows used by the truncation integration test.
CREATE TABLE dbo.large_table (id INT);
GO

WITH n AS (
    SELECT 1 AS i
    UNION ALL
    SELECT i + 1 FROM n WHERE i < 10001
)
INSERT INTO dbo.large_table (id)
SELECT i FROM n
OPTION (MAXRECURSION 0);
GO

CREATE VIEW dbo.user_summary AS
SELECT
    u.id,
    u.name,
    u.email,
    COUNT(o.id) AS order_count,
    COALESCE(SUM(o.amount), 0) AS total_spent
FROM dbo.users u
LEFT JOIN dbo.orders o ON u.id = o.user_id
GROUP BY u.id, u.name, u.email;
GO

-- Sentinel created LAST so the container healthcheck only passes once the
-- fixtures have finished loading (avoids tests racing an empty database).
CREATE TABLE dbo._dbridge_ready (ok BIT);
INSERT INTO dbo._dbridge_ready (ok) VALUES (1);
GO
