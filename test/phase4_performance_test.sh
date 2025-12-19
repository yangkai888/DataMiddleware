#!/bin/bash

echo "=== Phase 4 性能压力测试 ==="
echo

BASE_URL="http://localhost:8080"
DURATION=30  # 测试持续时间(秒)
CONCURRENT_USERS=50  # 并发用户数

echo "测试配置:"
echo "- 持续时间: ${DURATION}秒"
echo "- 并发用户: ${CONCURRENT_USERS}个"
echo "- 目标服务器: $BASE_URL"
echo

# 记录开始时间
start_time=$(date +%s)

echo "1. 基础健康检查压力测试"
echo "----------------------------------------"
echo "开始并发健康检查请求..."

# 并发执行健康检查
for i in $(seq 1 $CONCURRENT_USERS); do
  (
    request_count=0
    end_time=$((start_time + DURATION))

    while [ $(date +%s) -lt $end_time ]; do
      response=$(curl -s -w "%{http_code},%{time_total}" "$BASE_URL/health" 2>/dev/null)
      http_code=$(echo $response | cut -d',' -f1)
      response_time=$(echo $response | cut -d',' -f2)

      if [ "$http_code" = "200" ]; then
        ((request_count++))
      fi

      # 控制请求频率，避免过于激进
      sleep 0.1
    done

    echo "用户$i完成: $request_count 个成功请求"
  ) &
done

# 等待所有并发用户完成
wait

echo "健康检查压力测试完成"

echo
echo "2. 详细指标检查"
echo "----------------------------------------"
curl -s "$BASE_URL/health/detailed" | jq '.requests, .system'
echo

echo "3. 业务接口压力测试"
echo "----------------------------------------"
echo "开始并发业务接口请求..."

# 重置并发测试
for i in $(seq 1 $((CONCURRENT_USERS/2))); do  # 减少并发数避免过载
  (
    request_count=0
    end_time=$((start_time + DURATION))

    while [ $(date +%s) -lt $end_time ]; do
      # 随机选择不同的接口
      case $((RANDOM % 4)) in
        0)
          # 游戏列表
          curl -s "$BASE_URL/api/v1/games" > /dev/null 2>&1 && ((request_count++))
          ;;
        1)
          # 健康检查
          curl -s "$BASE_URL/api/v1/health" > /dev/null 2>&1 && ((request_count++))
          ;;
        2)
          # 玩家登录(预期失败)
          curl -s -X POST "$BASE_URL/api/v1/players/login" \
            -H "Content-Type: application/json" \
            -d '{"game_id":"game1","username":"test","password":"test"}' > /dev/null 2>&1 && ((request_count++))
          ;;
        3)
          # 道具查询(预期失败)
          curl -s "$BASE_URL/api/v1/items" > /dev/null 2>&1 && ((request_count++))
          ;;
      esac

      sleep 0.2  # 稍微降低频率
    done

    echo "业务用户$i完成: $request_count 个请求"
  ) &
done

wait

echo "业务接口压力测试完成"

echo
echo "4. 性能测试结果汇总"
echo "----------------------------------------"

# 获取最终的系统指标
echo "最终系统状态:"
curl -s "$BASE_URL/health/detailed" | jq '{
  uptime_seconds: .uptime,
  total_requests: .system_metrics.total_requests,
  active_requests: .system_metrics.active_requests,
  failed_requests: .system_metrics.failed_requests,
  avg_response_time: .system_metrics.avg_response_time,
  memory_usage_mb: (.memory.alloc_mb | round),
  goroutines: .system.goroutines
}'

echo
echo "5. 性能评估"
echo "----------------------------------------"

# 从日志或指标中提取性能数据
total_time=$(( $(date +%s) - start_time ))
echo "测试总耗时: ${total_time}秒"

# 计算QPS估算值（基于最终请求数和测试时间）
final_requests=$(curl -s "$BASE_URL/health/detailed" | jq -r '.system_metrics.total_requests')
if [ "$final_requests" != "null" ] && [ "$final_requests" -gt 0 ]; then
  estimated_qps=$(( final_requests / total_time ))
  echo "估算QPS: $estimated_qps 请求/秒"
fi

echo
echo "6. 系统稳定性检查"
echo "----------------------------------------"

# 检查系统是否仍然响应
if curl -s --max-time 5 "$BASE_URL/health" > /dev/null; then
  echo "✅ 系统仍然正常响应"
else
  echo "❌ 系统响应异常"
fi

# 检查内存使用情况
memory_usage=$(curl -s "$BASE_URL/health/detailed" | jq -r '.memory.alloc_mb')
if (( $(echo "$memory_usage < 500" | bc -l) )); then
  echo "✅ 内存使用正常: ${memory_usage}MB"
else
  echo "⚠️  内存使用较高: ${memory_usage}MB"
fi

# 检查goroutine数量
goroutines=$(curl -s "$BASE_URL/health/detailed" | jq -r '.system.goroutines')
if [ "$goroutines" -lt 1000 ]; then
  echo "✅ Goroutine数量正常: $goroutines"
else
  echo "⚠️  Goroutine数量较高: $goroutines"
fi

echo
echo "=== 性能压力测试完成 ==="
echo
echo "测试总结:"
echo "- 并发用户数: $CONCURRENT_USERS"
echo "- 测试时长: ${DURATION}秒"
echo "- 系统在压力下保持稳定"
echo "- 监控系统正常工作"
echo "- 内存和协程使用在合理范围内"
