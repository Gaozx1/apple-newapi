#!/bin/bash
echo "=== homepage content option (custom homepage override?) ==="
curl -s http://127.0.0.1:3000/api/home_page_content | head -c 300
echo ""
echo "=== binary has homepage card strings? ==="
docker exec new-api sh -c "grep -ac 'OAuth 2.0 Open Platform' /new-api"
docker exec new-api sh -c "grep -ac 'Calls / day' /new-api"
echo "=== sidebar personal oauth2 present? ==="
docker exec new-api sh -c "grep -ac 'OAuth 2.0 Apps' /new-api"