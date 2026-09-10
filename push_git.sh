#!/bin/bash
cd /data/data1/new-api-src
git init
git remote add gaozx1 https://github.com/Gaozx1/apple-newapi.git
git config user.name "Gaozx1"
git config user.email "gzx20140715@qq.com"
git add -A
git commit -m "feat: OAuth2.0 tiered billing + remove all rate limiting (v1.0.1)" --allow-empty
git push gaozx1 main --force 2>&1 || echo "PUSH_FAILED"