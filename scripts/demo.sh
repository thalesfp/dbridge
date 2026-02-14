#!/bin/bash
# Demo script for dbbridge

echo "=== dbbridge Demo ==="
echo ""

echo "1. Count all users:"
./dbbridge query "SELECT count(*) FROM users" --profile=local
echo ""

echo "2. List active user emails:"
./dbbridge query "SELECT email FROM users WHERE active = true LIMIT 5" --profile=local
echo ""

echo "3. Get user details (multi-column):"
./dbbridge query "SELECT id, name, email FROM users LIMIT 3" --profile=local
echo ""

echo "4. Aggregate query (active users and average age):"
./dbbridge query "SELECT COUNT(*) as active_users, AVG(age)::numeric(10,1) as avg_age FROM users WHERE active = true" --profile=local
echo ""

echo "5. List all schemas:"
./dbbridge schema list-schemas --profile=local
echo ""

echo "6. List tables in public schema:"
./dbbridge schema list-tables --schema=public --profile=local
echo ""

echo "7. Describe users table:"
./dbbridge schema describe users --schema=public --profile=local | head -20
echo "..."
echo ""

echo "8. Query from analytics schema:"
./dbbridge query "SELECT event_type, count(*) FROM analytics.events GROUP BY event_type" --profile=local
echo ""

echo "=== Demo Complete ==="
