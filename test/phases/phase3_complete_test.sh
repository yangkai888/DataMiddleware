#!/bin/bash

echo "=== Phase 3 业务逻辑层完整功能测试 ==="
echo

BASE_URL="http://localhost:8080/api/v1"
TEST_USER="testuser_$(date +%s)"
TEST_PASSWORD="testpass123"

echo "1. 健康检查测试"
curl -s "$BASE_URL/health" | jq '.status'
echo

echo "2. 玩家注册测试"
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/players/register" \
  -H "Content-Type: application/json" \
  -d "{\"game_id\":\"game1\",\"username\":\"$TEST_USER\",\"password\":\"$TEST_PASSWORD\",\"email\":\"$TEST_USER@example.com\"}")
echo "注册响应:"
echo "$REGISTER_RESPONSE" | jq '.'
USER_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.data.user_id')
echo "注册的用户ID: $USER_ID"
echo

echo "3. 玩家登录测试"
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/players/login" \
  -H "Content-Type: application/json" \
  -d "{\"game_id\":\"game1\",\"username\":\"$TEST_USER\",\"password\":\"$TEST_PASSWORD\"}")
echo "登录响应:"
echo "$LOGIN_RESPONSE" | jq '.'
ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.token.access_token')
echo "获取的访问令牌: ${ACCESS_TOKEN:0:50}..."
echo

if [ "$ACCESS_TOKEN" != "null" ] && [ "$ACCESS_TOKEN" != "" ]; then
    echo "4. 使用JWT令牌访问受保护接口"

    echo "4.1 获取玩家信息"
    curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
      "$BASE_URL/players/$USER_ID" | jq '.'
    echo

    echo "4.2 获取游戏列表"
    curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
      "$BASE_URL/games" | jq '.'
    echo

    echo "4.3 获取道具列表"
    curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
      "$BASE_URL/items?user_id=$USER_ID&game_id=game1" | jq '.'
    echo

    echo "4.4 创建道具"
    CREATE_ITEM_RESPONSE=$(curl -s -X POST "$BASE_URL/items" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"user_id\":\"$USER_ID\",\"game_id\":\"game1\",\"name\":\"测试道具\",\"type\":\"consumable\",\"quantity\":10}")
    echo "创建道具响应:"
    echo "$CREATE_ITEM_RESPONSE" | jq '.'
    echo

    echo "4.5 创建订单"
    CREATE_ORDER_RESPONSE=$(curl -s -X POST "$BASE_URL/orders" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"user_id\":\"$USER_ID\",\"game_id\":\"game1\",\"product_id\":\"prod001\",\"product_name\":\"钻石包\",\"amount\":9900,\"currency\":\"CNY\"}")
    echo "创建订单响应:"
    echo "$CREATE_ORDER_RESPONSE" | jq '.'
    ORDER_ID=$(echo "$CREATE_ORDER_RESPONSE" | jq -r '.data.order_id')
    echo

    echo "4.6 查询订单"
    curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
      "$BASE_URL/orders?user_id=$USER_ID" | jq '.'
    echo

    echo "5. 测试无认证访问（应该失败）"
    echo "5.1 无token访问游戏列表"
    curl -s "$BASE_URL/games" | jq '.code'
    echo

    echo "5.2 无效token访问"
    curl -s -H "Authorization: Bearer invalid_token" \
      "$BASE_URL/games" | jq '.code'
    echo
else
    echo "❌ 登录失败，跳过后续测试"
fi

echo "6. 游戏路由器测试（TCP）"
echo "测试TCP连接..."
echo "test_message" | nc -q 1 localhost 9090 && echo "✅ TCP连接成功" || echo "❌ TCP连接失败"
echo

echo "=== Phase 3 完整功能测试完成 ==="

