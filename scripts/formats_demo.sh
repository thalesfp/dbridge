#!/bin/bash
# Demo script showing all output formats

echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║           dbbridge Output Formats Demonstration                  ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1. COMPACT FORMAT (Ultra token-efficient for AI agents)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
./dbbridge query "SELECT id, name, email FROM users LIMIT 3" --profile=local --format=compact
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2. TABLE FORMAT (Beautiful Unicode borders)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
./dbbridge query "SELECT id, name, email FROM users LIMIT 3" --profile=local --format=table
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3. TABLE-COMPACT FORMAT (Plain text, easy to read)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
./dbbridge query "SELECT id, name, active FROM users LIMIT 3" --profile=local --format=table-compact
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4. CSV FORMAT (Standard CSV with headers)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
./dbbridge query "SELECT id, name, email FROM users LIMIT 3" --profile=local --format=csv
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "5. Smart Simplification (Compact format)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Count query (single value):"
./dbbridge query "SELECT count(*) FROM users" --profile=local --format=compact
echo ""
echo "Email list (single column):"
./dbbridge query "SELECT email FROM users WHERE active = true LIMIT 3" --profile=local --format=compact
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "6. Comparison: NULL and Boolean Handling"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Table format:"
./dbbridge query "SELECT name, age, active FROM users WHERE id <= 3" --profile=local --format=table
echo ""
echo "CSV format:"
./dbbridge query "SELECT name, age, active FROM users WHERE id <= 3" --profile=local --format=csv
echo ""

echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║                    Demo Complete!                             ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
