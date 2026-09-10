for i in 1 2 3 4 5; do
  curl -s -m 20 -o /dev/null -w "token#$i: HTTP:%{http_code} time:%{time_total}s\n" -X POST https://github.com/login/oauth/access_token -H "Accept: application/json" -d "client_id=x&client_secret=y&code=z" || echo "token#$i: FAILED"
done
echo "--- api.github.com/user ---"
for i in 1 2 3 4 5; do
  curl -s -m 20 -o /dev/null -w "api#$i: HTTP:%{http_code} time:%{time_total}s\n" https://api.github.com/user -H "Authorization: Bearer fake" -H "User-Agent: new-api" || echo "api#$i: FAILED"
done