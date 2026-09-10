#!/bin/bash
echo "=== rsbuild in log ==="
grep -c "build started" /data/data1/deploy_v102.log
echo "=== assets check ==="
docker exec new-api sh -c "ls /data/web 2>/dev/null || echo no-web-dir"
echo "=== version ==="
curl -s http://127.0.0.1:3000/api/status | grep -o "\"version\":\"[^\"]*\""
echo ""
echo "=== new frontend bundle contains user oauth2 endpoint? ==="
JS=$(curl -s http://127.0.0.1:3000/ | grep -o "/assets/index-[^\"]*\.js" | head -1)
echo "bundle: $JS"
curl -s "http://127.0.0.1:3000$JS" | grep -c "api/user/oauth2/clients"