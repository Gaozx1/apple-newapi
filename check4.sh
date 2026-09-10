#!/bin/bash
echo "=== all js chunks containing oauth2 clients endpoint ==="
for f in $(curl -s http://127.0.0.1:3000/ | grep -o "/static/js/[a-zA-Z0-9._-]*\.js" | sort -u); do
  n=$(curl -s "http://127.0.0.1:3000$f" | grep -c "user/oauth2/clients")
  if [ "$n" != "0" ]; then echo "$f -> $n"; fi
done
echo "=== index.html cache header ==="
curl -sI http://127.0.0.1:3000/ | grep -i "cache-control"