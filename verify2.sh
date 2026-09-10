#!/bin/bash
docker ps --format "{{.Image}} {{.Status}}" | grep new-api
echo "=== homepage section markers in binary ==="
docker exec new-api sh -c "grep -ac 'OAuth 2.0 Open Platform is live' /new-api"
docker exec new-api sh -c "grep -ac 'Start Building' /new-api"
echo "=== api health ==="
curl -s http://127.0.0.1:3000/api/status | grep -o "\"success\":true" | head -1