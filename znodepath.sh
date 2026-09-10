echo "=== node:20-slim 是否已拉全 ==="
docker images node:20-slim --format "{{.Repository}}:{{.Tag}} {{.Size}}"
echo "=== node 二进制位置 ==="
docker run --rm node:20-slim sh -c "which node; node --version; ls -l /usr/local/bin/node; echo 'libnode:'; ls /usr/local/lib/ 2>/dev/null | grep -i node"