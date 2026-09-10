echo "=== 本机含 node 的镜像 ==="
docker images | grep -iE "node|oven|bun" | head -10
echo "--- 全部镜像名 ---"
docker images --format "{{.Repository}}:{{.Tag}}" | head -40
echo "=== 再测几个 pypi/npm 源 ==="
curl -s -m 6 -o /dev/null -w "aliyun-pypi: %{http_code} %{time_total}s\n" https://mirrors.aliyun.com/pypi/simple/ || echo "aliyun-pypi FAIL"
curl -s -m 6 -o /dev/null -w "ustc-pypi: %{http_code} %{time_total}s\n" https://mirrors.ustc.edu.cn/pypi/simple/ || echo "ustc-pypi FAIL"
curl -s -m 6 -o /dev/null -w "npmmirror2: %{http_code} %{time_total}s\n" https://registry.npmmirror.com/-/binary/node/ || echo "npmmirror2 FAIL"