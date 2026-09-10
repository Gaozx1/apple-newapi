#!/bin/bash
echo "=== source tar has new api.ts? ==="
grep -c "user/oauth2/clients" /data/data1/new-api-src/web/src/features/oauth2/api.ts
echo "=== embedded binary has new string? ==="
grep -ac "user/oauth2/clients" /new-api 2>/dev/null || echo "binary check failed"
docker exec new-api sh -c "grep -ac 'user/oauth2/clients' /new-api" 2>/dev/null || echo "exec failed"
echo "=== docs tab string in binary? ==="
docker exec new-api sh -c "grep -ac 'Free of charge' /new-api" 2>/dev/null