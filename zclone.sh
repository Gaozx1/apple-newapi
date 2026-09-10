cd /root
rm -rf zcode2api-new
echo "=== 尝试克隆 yourhoneypomelo-cell/zcode2api ==="
for m in "https://ghfast.top/https://github.com" "https://gh-proxy.com/https://github.com"; do
  echo "--- 用镜像: $m ---"
  if timeout 120 git clone --depth 1 "$m/yourhoneypomelo-cell/zcode2api" zcode2api-new 2>&1 | tail -3; then
    echo "CLONE_OK"
    break
  else
    echo "此镜像失败，试下一个"
  fi
done
echo "=== 克隆结果 ==="
if [ -d /root/zcode2api-new/.git ]; then
  cd /root/zcode2api-new
  echo "commit: $(git log --oneline -3 | head -3)"
  echo "--- 结构 ---"
  ls -la
else
  echo "CLONE_FAILED"
fi