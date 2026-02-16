#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="${SCRIPT_DIR}/certs"

mkdir -p "${CERT_DIR}"

if [ -f "${CERT_DIR}/server.crt" ] && [ -f "${CERT_DIR}/server.key" ]; then
    echo "SSL certs already exist in ${CERT_DIR}, skipping generation."
    exit 0
fi

echo "Generating self-signed SSL certificates..."
openssl req -new -x509 -days 365 -nodes \
    -out "${CERT_DIR}/server.crt" \
    -keyout "${CERT_DIR}/server.key" \
    -subj "/CN=localhost"

chmod 600 "${CERT_DIR}/server.key"
echo "SSL certificates generated in ${CERT_DIR}"
