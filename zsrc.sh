pkill -f "docker build.*zcode2api" 2>/dev/null
sleep 1
echo "=== 残留构建进程 ==="
ps aux | grep "docker build" | grep -v grep | head -3
echo "(以上为空则已清理)"
echo "=== 国内源连通性 ==="
curl -s -m 8 -o /dev/null -w "tuna-pypi: %{http_code} %{time_total}s\n" https://pypi.tuna.tsinghua.edu.cn/simple/ || echo "tuna-pypi FAIL"
curl -s -m 8 -o /dev/null -w "tuna-debian: %{http_code} %{time_total}s\n" https://mirrors.tuna.tsinghua.edu.cn/debian/ || echo "tuna-debian FAIL"
curl -s -m 8 -o /dev/null -w "npm-taobao: %{http_code} %{time_total}s\n" https://registry.npmmirror.com/ || echo "npm-taobao FAIL"
curl -s -m 8 -o /dev/null -w "deb-nodesource: %{http_code} %{time_total}s\n" https://deb.nodesource.com/setup_20.x || echo "nodesource FAIL"