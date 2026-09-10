#!/bin/bash
echo "=== script tags on homepage ==="
curl -s http://127.0.0.1:3000/ | grep -o "src=\"[^\"]*\.js\"" | head -5
echo "=== search all asset js for new endpoint ==="
for f in $(curl -s http://127.0.0.1:3000/ | grep -o "/assets/[^\"]*\.js" | sort -u); do
  n=$(curl -s "http://127.0.0.1:3000$f" | grep -c "user/oauth2/clients")
  echo "$f -> $n"
done