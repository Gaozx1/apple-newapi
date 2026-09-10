cd /root/zcode2api-new
echo "========== Dockerfile =========="
cat Dockerfile
echo "========== requirements.txt =========="
cat requirements.txt
echo "========== docker-compose.yml =========="
cat docker-compose.yml
echo "========== .env.example 关键项 =========="
grep -E "ZCODE_PORT|ZCODE_HOST|ZCODE_DATA_DIR|ZCODE_ADMIN_KEY|ZCODE_NODE" .env.example | head -10
echo "========== main.py 端口逻辑 =========="
grep -nE "ZCODE_PORT|settings.PORT|--port|uvicorn" main.py | head -8