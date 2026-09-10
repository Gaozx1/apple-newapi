echo "=== 测 docker 拉 node:20-slim (走加速器) ==="
timeout 90 docker pull node:20-slim 2>&1 | tail -5
echo "PULL_EXIT:$?"
echo "=== 测 npmmirror node 二进制下载速度 ==="
curl -s -m 15 -o /dev/null -w "node-tarball: %{http_code} %{time_total}s size_dl=%{size_download}\n" "https://registry.npmmirror.com/-/binary/node/v20.18.0/node-v20.18.0-linux-x64.tar.xz" -r 0-1000000 || echo "FAIL"