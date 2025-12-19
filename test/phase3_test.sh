#!/bin/bash

echo "=== Phase 3 业务逻辑层功能测试 ==="
echo

BASE_URL="http://localhost:8080/api/v1"

echo "1. 健康检查测试"
curl -s "$BASE_URL/health" | jq '.status'
echo

echo "2. 游戏列表测试"
curl -s "$BASE_URL/games" | jq '.data.games[0].game_id'
echo

echo "3. 玩家登录测试（预期失败 - 无数据库）"
curl -s -X POST "$BASE_URL/players/login" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"test001","game_id":"game1","device_id":"device123","platform":"android","version":"1.0.0"}' | jq '.code'
echo

echo "4. 道具创建测试（预期失败 - 无数据库）"
curl -s -X POST "$BASE_URL/items" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"test001","game_id":"game1","name":"金币","type":"currency","category":"resource","quantity":100}' | jq '.code'
echo

echo "5. 订单创建测试（模拟成功）"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"test001","game_id":"game1","product_id":"prod001","product_name":"钻石包","amount":9900,"currency":"CNY"}' | jq '.message'
echo

echo "6. 订单查询测试（模拟数据）"
curl -s "$BASE_URL/orders?user_id=test001" | jq '.data.orders | length'
echo

echo "7. TCP连接测试"
echo "test" | nc -q 1 localhost 9090 && echo "TCP连接成功" || echo "TCP连接失败"
echo

echo "=== Phase 3 测试完成 ==="
