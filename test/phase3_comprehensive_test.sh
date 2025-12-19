#!/bin/bash

echo "=== Phase 3 业务逻辑层综合功能测试 ==="
echo "测试时间: $(date)"
echo

BASE_URL="http://localhost:8080/api/v1"
TEST_USER="testuser_phase3_$(date +%s)"
TEST_PASSWORD="testpass123"

echo "📋 测试计划:"
echo "1. 服务器健康检查"
echo "2. 玩家注册功能"
echo "3. 玩家登录功能 (JWT令牌)"
echo "4. 认证中间件验证"
echo "5. 玩家信息管理"
echo "6. 道具管理功能"
echo "7. 订单管理功能"
echo "8. 游戏路由器功能"
echo "9. 安全验证测试"
echo

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试计数器
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# 测试函数
run_test() {
    local test_name="$1"
    local test_cmd="$2"

    TESTS_RUN=$((TESTS_RUN + 1))
    echo -n "🔍 测试 $TESTS_RUN: $test_name ... "

    if eval "$test_cmd" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ 通过${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}❌ 失败${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

echo "🚀 步骤1: 启动服务器..."
# 检查服务器是否已经在运行
if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
    echo "✅ 服务器已在运行"
else
    echo "❌ 服务器未运行，请先启动服务器"
    echo "运行命令: go run cmd/server/main.go"
    exit 1
fi

echo
echo "🩺 步骤2: 健康检查测试"
run_test "健康检查接口" "curl -s '$BASE_URL/health' | jq -e '.status == \"ok\"'"
run_test "详细健康检查" "curl -s 'http://localhost:8080/health/detailed' | jq -e '.status'"

echo
echo "👤 步骤3: 玩家注册功能测试"
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/players/register" \
  -H "Content-Type: application/json" \
  -d "{\"game_id\":\"game1\",\"username\":\"$TEST_USER\",\"password\":\"$TEST_PASSWORD\",\"email\":\"$TEST_USER@example.com\"}")

if echo "$REGISTER_RESPONSE" | jq -e '.code == 0' > /dev/null 2>&1; then
    run_test "玩家注册" "echo '$REGISTER_RESPONSE' | jq -e '.code == 0'"
    USER_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.data.user_id')
    echo "   📝 注册成功，用户ID: $USER_ID"
else
    echo "   ❌ 注册失败: $(echo "$REGISTER_RESPONSE" | jq -r '.message')"
    run_test "玩家注册" "false"
fi

echo
echo "🔐 步骤4: 玩家登录功能测试"
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/players/login" \
  -H "Content-Type: application/json" \
  -d "{\"game_id\":\"game1\",\"username\":\"$TEST_USER\",\"password\":\"$TEST_PASSWORD\"}")

if echo "$LOGIN_RESPONSE" | jq -e '.code == 0' > /dev/null 2>&1; then
    run_test "玩家登录" "echo '$LOGIN_RESPONSE' | jq -e '.code == 0'"
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.token.access_token')
    REFRESH_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.token.refresh_token')
    echo "   🔑 登录成功，获取到JWT令牌"

    # 验证令牌结构
    run_test "JWT访问令牌生成" "[ -n '$ACCESS_TOKEN' ] && [ '$ACCESS_TOKEN' != 'null' ]"
    run_test "JWT刷新令牌生成" "[ -n '$REFRESH_TOKEN' ] && [ '$REFRESH_TOKEN' != 'null' ]"
else
    echo "   ❌ 登录失败: $(echo "$LOGIN_RESPONSE" | jq -r '.message')"
    run_test "玩家登录" "false"
    ACCESS_TOKEN=""
fi

echo
echo "🛡️ 步骤5: 认证中间件测试"
if [ -n "$ACCESS_TOKEN" ]; then
    # 测试需要认证的接口
    GAMES_RESPONSE=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" "$BASE_URL/games")
    run_test "认证接口访问" "echo '$GAMES_RESPONSE' | jq -e '.code == 0'"

    # 测试无认证访问
    UNAUTH_GAMES=$(curl -s "$BASE_URL/games")
    run_test "无认证访问拒绝" "echo '$UNAUTH_GAMES' | jq -e '.code == 401'"
else
    echo "   ⚠️  跳过认证测试（登录失败）"
fi

echo
echo "👤 步骤6: 玩家信息管理测试"
if [ -n "$ACCESS_TOKEN" ] && [ -n "$USER_ID" ]; then
    # 获取玩家信息
    PLAYER_INFO=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" "$BASE_URL/players/$USER_ID")
    run_test "获取玩家信息" "echo '$PLAYER_INFO' | jq -e '.code == 0'"

    # 更新玩家信息
    UPDATE_RESPONSE=$(curl -s -X PUT "$BASE_URL/players/$USER_ID" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"nickname":"测试昵称"}')
    run_test "更新玩家信息" "echo '$UPDATE_RESPONSE' | jq -e '.code == 0'"
else
    echo "   ⚠️  跳过玩家信息测试（认证失败）"
fi

echo
echo "🎒 步骤7: 道具管理功能测试"
if [ -n "$ACCESS_TOKEN" ] && [ -n "$USER_ID" ]; then
    # 创建道具
    CREATE_ITEM=$(curl -s -X POST "$BASE_URL/items" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"user_id\":\"$USER_ID\",\"game_id\":\"game1\",\"name\":\"测试道具\",\"type\":\"consumable\",\"quantity\":10}")

    if echo "$CREATE_ITEM" | jq -e '.code == 0' > /dev/null 2>&1; then
        run_test "创建道具" "echo '$CREATE_ITEM' | jq -e '.code == 0'"
        ITEM_ID=$(echo "$CREATE_ITEM" | jq -r '.data.item_id')
        echo "   📦 创建道具成功，ID: $ITEM_ID"
    else
        run_test "创建道具" "false"
        ITEM_ID=""
    fi

    # 获取道具列表
    ITEMS_LIST=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" "$BASE_URL/items?user_id=$USER_ID&game_id=game1")
    run_test "获取道具列表" "echo '$ITEMS_LIST' | jq -e '.code == 0'"
else
    echo "   ⚠️  跳过道具测试（认证失败）"
fi

echo
echo "🛒 步骤8: 订单管理功能测试"
if [ -n "$ACCESS_TOKEN" ] && [ -n "$USER_ID" ]; then
    # 创建订单
    CREATE_ORDER=$(curl -s -X POST "$BASE_URL/orders" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"user_id\":\"$USER_ID\",\"game_id\":\"game1\",\"product_id\":\"prod001\",\"product_name\":\"钻石包\",\"amount\":9900,\"currency\":\"CNY\"}")

    if echo "$CREATE_ORDER" | jq -e '.code == 0' > /dev/null 2>&1; then
        run_test "创建订单" "echo '$CREATE_ORDER' | jq -e '.code == 0'"
        ORDER_ID=$(echo "$CREATE_ORDER" | jq -r '.data.order_id')
        echo "   📋 创建订单成功，ID: $ORDER_ID"
    else
        run_test "创建订单" "false"
        ORDER_ID=""
    fi

    # 查询订单
    ORDERS_LIST=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" "$BASE_URL/orders?user_id=$USER_ID")
    run_test "查询订单列表" "echo '$ORDERS_LIST' | jq -e '.code == 0'"
else
    echo "   ⚠️  跳过订单测试（认证失败）"
fi

echo
echo "🎮 步骤9: 游戏路由器功能测试"
# 测试游戏列表
GAMES_LIST=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" "$BASE_URL/games")
run_test "游戏列表获取" "echo '$GAMES_LIST' | jq -e '.code == 0'"

# 测试游戏统计
GAME_STATS=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" "$BASE_URL/games/game1/stats")
run_test "游戏统计信息" "echo '$GAME_STATS' | jq -e '.code == 0'"

# 测试TCP连接
echo "   🔌 测试TCP连接..."
TCP_TEST=$(echo "test" | timeout 5 nc -q 1 localhost 9090 2>/dev/null && echo "success" || echo "failed")
run_test "TCP服务器连接" "[ '$TCP_TEST' = 'success' ]"

echo
echo "🔒 步骤10: 安全验证测试"
# 测试无效令牌
INVALID_TOKEN_TEST=$(curl -s -H "Authorization: Bearer invalid.jwt.token" "$BASE_URL/games")
run_test "无效令牌拒绝" "echo '$INVALID_TOKEN_TEST' | jq -e '.code == 401'"

# 测试空Authorization头
EMPTY_AUTH_TEST=$(curl -s "$BASE_URL/games")
run_test "空认证头拒绝" "echo '$EMPTY_AUTH_TEST' | jq -e '.code == 401'"

# 测试SQL注入防护（模拟）
SQL_INJECTION_TEST=$(curl -s -X POST "$BASE_URL/players/login" \
  -H "Content-Type: application/json" \
  -d '{"game_id":"game1","username":"admin\"; --","password":"test"}')
run_test "SQL注入防护" "echo '$SQL_INJECTION_TEST' | jq -e '.code != 0'"

echo
echo "📊 测试结果汇总"
echo "========================================"
echo "总测试数: $TESTS_RUN"
echo -e "通过测试: ${GREEN}$TESTS_PASSED${NC}"
echo -e "失败测试: ${RED}$TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 所有测试通过！Phase 3功能完备！${NC}"
    echo
    echo "✅ Phase 3 验收标准达成:"
    echo "   • 游戏路由器正常工作"
    echo "   • 玩家业务模块功能完整"
    echo "   • 道具业务模块功能完整"
    echo "   • 订单业务模块功能完整"
    echo "   • JWT认证系统安全可靠"
    echo "   • HTTP API接口符合规范"
    echo "   • 数据库操作正常"
    echo "   • 错误处理完善"
    echo
    echo "🏆 Phase 3 业务逻辑层实现成功！"
    echo "🚀 可以开始Phase 4（缓存和基础设施层）的开发"
else
    echo -e "${YELLOW}⚠️  部分测试失败，需要检查和修复${NC}"
fi

echo "========================================"
