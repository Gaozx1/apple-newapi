#!/bin/bash
docker ps --format "{{.Image}} {{.Status}}" | grep new-api
echo "=== scroll fix marker in binary ==="
docker exec new-api sh -c "grep -ac 'Per user, resets daily' /new-api"
echo "=== api health ==="
curl -s http://127.0.0.1:3000/api/status | grep -o "\"version\":\"[^\"]*\""