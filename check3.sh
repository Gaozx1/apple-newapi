#!/bin/bash
for f in index.0de661b021.js 2078.449009cbaf.js; do
  n=$(curl -s "http://127.0.0.1:3000/static/js/$f" | grep -c "user/oauth2/clients")
  echo "$f -> $n"
done