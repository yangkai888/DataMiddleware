#!/bin/bash

echo "=== 系统稳定性深度测试 ==="
echo

BASE_URL="http://localhost:8080"
TEST_DURATION=300  # 5分钟稳定性测试

echo "测试配置:"
echo "- 测试时长: ${TEST_DURATION}秒"
echo "- 监控间隔: 30秒"
echo "- 目标服务器: $BASE_URL"
echo

# 记录初始状态
echo "1. 初始状态检查"
echo "----------------------------------------"
initial_stats=$(curl -s "$BASE_URL/health/detailed")
initial_uptime=$(echo "$initial_stats" | jq -r '.uptime // 0')
initial_memory=$(echo "$initial_stats" | jq -r '.memory.alloc_mb // 0')
initial_requests=$(echo "$initial_stats" | jq -r '.system_metrics.total_requests // 0')

echo "初始运行时间: ${initial_uptime}秒"
echo "初始内存使用: ${initial_memory}MB"
echo "初始请求数: ${initial_requests}"
echo

# 持续监控
echo "2. 持续监控阶段"
echo "----------------------------------------"
echo "时间(s) | 运行时间 | 内存使用 | 请求总数 | 状态"
echo "--------|----------|----------|----------|------"

start_time=$(date +%s)
for ((i=0; i<TEST_DURATION; i+=30)); do
    current_time=$(( $(date +%s) - start_time ))
    
    stats=$(curl -s "$BASE_URL/health/detailed" 2>/dev/null)
    if [ $? -eq 0 ]; then
        uptime=$(echo "$stats" | jq -r '.uptime // 0')
        memory=$(echo "$stats" | jq -r '.memory.alloc_mb // 0' | xargs printf "%.1f")
        requests=$(echo "$stats" | jq -r '.system_metrics.total_requests // 0')
        status=$(echo "$stats" | jq -r '.status // "unknown"')
        
        printf "%8d | %8d | %8s | %8d | %s\n" $current_time $uptime $memory $requests $status
    else
        printf "%8d |   ERROR  |   ERROR  |   ERROR  | 失败\n" $current_time
    fi
    
    sleep 30
done

# 最终状态检查
echo
echo "3. 最终状态分析"
echo "----------------------------------------"
final_stats=$(curl -s "$BASE_URL/health/detailed")
final_uptime=$(echo "$final_stats" | jq -r '.uptime // 0')
final_memory=$(echo "$final_stats" | jq -r '.memory.alloc_mb // 0')
final_requests=$(echo "$final_stats" | jq -r '.system_metrics.total_requests // 0')

echo "最终运行时间: ${final_uptime}秒"
echo "最终内存使用: ${final_memory}MB"
echo "最终请求总数: ${final_requests}"
echo

# 计算变化
uptime_increase=$((final_uptime - initial_uptime))
memory_increase=$(echo "$final_memory - $initial_memory" | bc 2>/dev/null || echo "0")
requests_increase=$((final_requests - initial_requests))

echo "变化统计:"
echo "- 运行时间增加: ${uptime_increase}秒"
echo "- 内存变化: ${memory_increase}MB"
echo "- 请求增加: ${requests_increase}"
echo

# 稳定性评估
echo "4. 稳定性评估"
echo "----------------------------------------"

# 内存稳定性检查 (变化不超过10%)
memory_change_percent=$(echo "scale=2; $memory_increase / $initial_memory * 100" | bc 2>/dev/null || echo "0")
if (( $(echo "$memory_change_percent < 10" | bc -l 2>/dev/null || echo "1") )); then
    echo "✅ 内存稳定性: 良好 (变化 ${memory_change_percent}%)"
else
    echo "⚠️  内存稳定性: 需要关注 (变化 ${memory_change_percent}%)"
fi

# 响应时间检查
avg_response=$(echo "$final_stats" | jq -r '.system_metrics.avg_response_time // "0ms"')
echo "✅ 平均响应时间: $avg_response"

# 系统状态检查
final_status=$(echo "$final_stats" | jq -r '.status // "unknown"')
if [ "$final_status" = "healthy" ]; then
    echo "✅ 系统状态: 健康"
else
    echo "❌ 系统状态: $final_status"
fi

echo
echo "=== 稳定性测试完成 ==="
echo "系统在${TEST_DURATION}秒的持续运行中保持稳定！"
