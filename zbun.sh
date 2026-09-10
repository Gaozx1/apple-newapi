echo "=== oven/bun:1 里的 node/bun ==="
docker run --rm oven/bun:1 sh -c "which node; which bun; node --version 2>/dev/null; bun --version 2>/dev/null; echo 'node路径:'; ls -l \$(which node) 2>/dev/null"
echo "=== captcha_node/package.json 依赖 ==="
cat /root/zcode2api/captcha_node/package.json
echo "=== 有无 package-lock ==="
ls -l /root/zcode2api/captcha_node/package-lock.json 2>/dev/null || echo "无lock文件"