package main

import (
	"fmt"
	"net/http"
	"time"
)

// 简单性能测试 - 先验证服务器基本功能
func main() {
	fmt.Println("🔍 简单性能测试 - 验证服务器基本功能")
	fmt.Println("=====================================")

	baseURL := "http://localhost:8080"

	// 测试1: 健康检查
	fmt.Println("\n1. 健康检查测试")
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		fmt.Printf("❌ 健康检查失败: %v\n", err)
		return
	}
	resp.Body.Close()
	fmt.Printf("✅ 健康检查成功, 状态码: %d\n", resp.StatusCode)

	// 测试2: 玩家登录API
	fmt.Println("\n2. 玩家登录API测试")
	testPlayerLogin(baseURL)

	// 测试3: 获取玩家信息
	fmt.Println("\n3. 获取玩家信息API测试")
	testPlayerInfo(baseURL)

	// 测试4: 道具列表API
	fmt.Println("\n4. 道具列表API测试")
	testItemList(baseURL)

	fmt.Println("\n🎉 基础功能测试完成！")
}

// testPlayerLogin 测试玩家登录
func testPlayerLogin(baseURL string) {
	url := baseURL + "/api/game1/player/login"
	fmt.Printf("测试URL: %s\n", url)

	start := time.Now()
	resp, err := http.Post(url, "application/json", nil)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("✅ 响应时间: %v, 状态码: %d\n", duration, resp.StatusCode)
}

// testPlayerInfo 测试获取玩家信息
func testPlayerInfo(baseURL string) {
	url := baseURL + "/api/game1/player/1001"
	fmt.Printf("测试URL: %s\n", url)

	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("✅ 响应时间: %v, 状态码: %d\n", duration, resp.StatusCode)
}

// testItemList 测试道具列表
func testItemList(baseURL string) {
	url := baseURL + "/api/game1/player/1001/items"
	fmt.Printf("测试URL: %s\n", url)

	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("✅ 响应时间: %v, 状态码: %d\n", duration, resp.StatusCode)
}
