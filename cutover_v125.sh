#!/bin/sh
# Minimal-downtime swap of the production new-api container to the v1.2.5 image.
# A probe loop keeps hitting /api/status so the observed gap is measured, not guessed.
set -u

PROBE=/tmp/cutover_probe_v125.log
IMAGE=new-api-enc:v1.2.5
ENVFILE=/data/data1/newapi_v125.env

echo "--- create the replacement container before the swap ---"
docker rm -f new-api-next >/dev/null 2>&1
docker create --name new-api-next --restart always --network host \
  --env-file "$ENVFILE" -v /data/data1/new-api:/data "$IMAGE"

echo "--- probe every 100ms while switching ---"
rm -f "$PROBE"
nohup sh -c 'i=0; while [ $i -lt 400 ]; do printf "%s %s\n" "$(date +%s.%N)" "$(curl -s -o /dev/null -w "%{http_code}" -m 2 http://127.0.0.1:3000/api/status)" >> '"$PROBE"'; i=$((i+1)); done' >/dev/null 2>&1 &
sleep 2

echo "--- swap ---"
SWAP_START=$(date +%s.%N)
docker stop new-api && docker rm new-api
docker rename new-api-next new-api && docker start new-api
SWAP_END=$(date +%s.%N)
echo "swap commands took: $(echo "$SWAP_END - $SWAP_START" | bc) s"

sleep 8
echo "--- production health ---"
curl -s -m 10 http://127.0.0.1:3000/api/status | head -c 160
echo
docker ps --format '{{.Names}} | {{.Image}} | {{.Status}}' | grep new-api
echo "--- new image boot log tail ---"
docker logs --tail 12 new-api 2>&1 | tr -d '\r'
echo "--- probe result (timestamp code) ---"
cat "$PROBE"
