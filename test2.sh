#!/bin/bash
for i in $(seq 1 5); do
  body=$(curl -s -X POST http://127.0.0.1:3000/api/user/login -H 'Content-Type: application/json' -d '{}')
  echo "req$i: $body"
done