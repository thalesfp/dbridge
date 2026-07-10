#!/bin/bash
set -euo pipefail

module="$(go list -m)"
dependencies="$(go list -deps ./cmd/dbridge)"

if grep -Fxq "$module/internal/writedb" <<<"$dependencies"; then
  echo "FAIL: cmd/dbridge depends on internal/writedb" >&2
  exit 1
fi

if grep -Fxq "$module/internal/writecli" <<<"$dependencies"; then
  echo "FAIL: cmd/dbridge depends on internal/writecli" >&2
  exit 1
fi
