cd /root/zcode2api
echo "=== 结构 ==="
ls -la
echo "=== 关键文件检测 ==="
for f in Dockerfile docker-compose.yml compose.yml package.json requirements.txt go.mod README.md .env.example .env app.py main.py index.js server.py main.go; do
  if [ -e "$f" ]; then echo "有: $f"; fi
done
echo "=== 一级子目录 ==="
find . -maxdepth 1 -type d | head -30