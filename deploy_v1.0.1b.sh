#!/bin/bash
set -e

cd /data/data1
rm -rf new-api-src
mkdir new-api-src
cd new-api-src
tar xzf /data/data1/deploy_v1.0.1b.tar.gz

docker build --network=host -t new-api-enc:v1.0.1 .

docker stop new-api 2>/dev/null || true
docker rm new-api 2>/dev/null || true

docker run -d --name new-api --restart always --network host \
  -e TZ=Asia/Shanghai \
  -e "SQL_DSN=newapi_app:018d532f25992337c3d52115426562301a1159ffc5e212ae@tcp(127.0.0.1:3306)/newapi?charset=utf8mb4&parseTime=True&loc=Local" \
  -e "TRUSTED_PROXIES=173.245.48.0/20,103.21.244.0/22,103.22.200.0/22,103.31.4.0/22,141.101.64.0/18,108.162.192.0/18,190.93.240.0/20,188.114.96.0/20,197.234.240.0/22,198.41.128.0/17,162.158.0.0/15,104.16.0.0/12,172.64.0.0/13,131.0.72.0/22,43.248.3.158" \
  -v /data/data1/new-api:/data \
  new-api-enc:v1.0.1

echo "DEPLOY DONE"
docker ps | grep new-api