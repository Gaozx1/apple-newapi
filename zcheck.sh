echo "=== 3001 端口占用情况 ==="
(ss -tlnp 2>/dev/null | grep -w 3001) || echo "3001 空闲"
echo "=== 3000 端口 ==="
(ss -tlnp 2>/dev/null | grep -w 3000) || echo "3000 空闲"
echo "=== main.py 端口/数据目录相关 ==="
grep -nE "ZCODE_PORT|ZCODE_HOST|ZCODE_DATA_DIR|uvicorn|port|DATA_DIR" /root/zcode2api/main.py | head -30
echo "=== docker-compose 可用? ==="
(docker compose version 2>/dev/null || docker-compose version 2>/dev/null) || echo "无 compose"