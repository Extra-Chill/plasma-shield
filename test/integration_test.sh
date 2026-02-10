#!/bin/bash
# Integration test for Plasma Shield proxy
set -e

PROXY_PORT=18080
API_PORT=18081

echo "=== Plasma Shield Integration Test ==="

# Build
echo "Building..."
cd "$(dirname "$0")/.."
go build -o bin/plasma-shield-router ./cmd/proxy

# Create test rules
cat > /tmp/test-rules.yaml << 'EOF'
rules:
  - id: block-evil
    domain: "evil.com"
    action: block
    description: "Block evil.com"
    enabled: true
  - id: block-crypto
    domain: "*.crypto-mining.net"
    action: block
    description: "Block crypto mining"
    enabled: true
EOF

# Start proxy in background
echo "Starting proxy on :$PROXY_PORT (API on :$API_PORT)..."
./bin/plasma-shield-router \
  --proxy-addr ":$PROXY_PORT" \
  --api-addr ":$API_PORT" \
  --rules /tmp/test-rules.yaml &
PROXY_PID=$!

# Wait for startup
sleep 2

# Cleanup on exit
cleanup() {
  echo "Cleaning up..."
  kill $PROXY_PID 2>/dev/null || true
}
trap cleanup EXIT

echo ""
echo "=== Test 1: Health check ==="
HEALTH=$(curl -s http://localhost:$API_PORT/health)
if [ "$HEALTH" = "OK" ]; then
  echo "✓ Health check passed"
else
  echo "✗ Health check failed: $HEALTH"
  exit 1
fi

echo ""
echo "=== Test 2: Mode check ==="
MODE=$(curl -s http://localhost:$API_PORT/mode | jq -r .global_mode)
if [ "$MODE" = "enforce" ]; then
  echo "✓ Default mode is enforce"
else
  echo "✗ Unexpected mode: $MODE"
  exit 1
fi

echo ""
echo "=== Test 3: Allowed domain (HTTP proxy) ==="
# Request to httpbin.org should succeed
RESULT=$(curl -s -o /dev/null -w "%{http_code}" --proxy http://localhost:$PROXY_PORT http://httpbin.org/get 2>/dev/null || echo "000")
if [ "$RESULT" = "200" ]; then
  echo "✓ Allowed domain returns 200"
else
  echo "✗ Expected 200, got: $RESULT"
  exit 1
fi

echo ""
echo "=== Test 4: Blocked domain ==="
# Request to evil.com should be blocked
RESULT=$(curl -s -o /dev/null -w "%{http_code}" --proxy http://localhost:$PROXY_PORT http://evil.com/ 2>/dev/null || echo "000")
if [ "$RESULT" = "403" ]; then
  echo "✓ Blocked domain returns 403"
else
  echo "✗ Expected 403, got: $RESULT"
  exit 1
fi

echo ""
echo "=== Test 5: Set mode to lockdown ==="
curl -s -X PUT http://localhost:$API_PORT/mode -d '{"mode":"lockdown"}' > /dev/null
MODE=$(curl -s http://localhost:$API_PORT/mode | jq -r .global_mode)
if [ "$MODE" = "lockdown" ]; then
  echo "✓ Mode changed to lockdown"
else
  echo "✗ Mode change failed: $MODE"
  exit 1
fi

echo ""
echo "=== Test 6: Lockdown blocks everything ==="
RESULT=$(curl -s -o /dev/null -w "%{http_code}" --proxy http://localhost:$PROXY_PORT http://httpbin.org/get 2>/dev/null || echo "000")
if [ "$RESULT" = "403" ]; then
  echo "✓ Lockdown blocks all traffic"
else
  echo "✗ Expected 403 in lockdown, got: $RESULT"
  exit 1
fi

echo ""
echo "=== Test 7: Set mode to audit ==="
curl -s -X PUT http://localhost:$API_PORT/mode -d '{"mode":"audit"}' > /dev/null
MODE=$(curl -s http://localhost:$API_PORT/mode | jq -r .global_mode)
if [ "$MODE" = "audit" ]; then
  echo "✓ Mode changed to audit"
else
  echo "✗ Mode change failed: $MODE"
  exit 1
fi

echo ""
echo "=== Test 8: Audit allows blocked domains ==="
RESULT=$(curl -s -o /dev/null -w "%{http_code}" --proxy http://localhost:$PROXY_PORT http://evil.com/ 2>/dev/null || echo "000")
# In audit mode, blocked domains should NOT return 403
# They might fail for other reasons (DNS, connection) but not 403
if [ "$RESULT" != "403" ]; then
  echo "✓ Audit mode does not block (got $RESULT)"
else
  echo "✗ Audit mode should not block, got: 403"
  exit 1
fi

echo ""
echo "=== All tests passed! ==="
