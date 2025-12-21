package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"
	"time"
)

func main() {
	fmt.Println("🔍 TCP协议简单测试")

	// 构建TCP消息
	message := buildTCPMessage(1001, []byte(`{"player_id":1001,"action":"login","game_id":"game1"}`), "game1", "user1001")

	fmt.Printf("消息长度: %d 字节\n", len(message))
	fmt.Printf("消息内容: %x\n", message)

	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:9090")
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	// 设置超时
	conn.SetWriteTimeout(5 * time.Second)
	conn.SetReadTimeout(5 * time.Second)

	// 发送消息
	_, err = conn.Write(message)
	if err != nil {
		fmt.Printf("❌ 发送失败: %v\n", err)
		return
	}

	fmt.Println("✅ 消息发送成功")

	// 读取响应
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 收到响应: %d 字节\n", n)
	fmt.Printf("响应内容: %x\n", buffer[:n])
}

// buildTCPMessage 构建TCP消息 (二进制协议格式，与BinaryCodec.Encode完全一致)
func buildTCPMessage(msgType uint16, body []byte, gameID, userID string) []byte {
	gameIDBytes := []byte(gameID)
	userIDBytes := []byte(userID)

	// 计算消息总长度
	gameIDLen := uint16(len(gameIDBytes))
	userIDLen := uint16(len(userIDBytes))
	bodyLen := uint32(len(body))

	// 固定头部长度: 版本(1) + 类型(2) + 标志(1) + 序列号(4) + 时间戳(8) + 体长度(4) + 校验和(4) + 游戏ID长度(2) + 用户ID长度(2)
	fixedHeaderLen := 1 + 2 + 1 + 4 + 8 + 4 + 4 + 2 + 2
	totalLen := fixedHeaderLen + int(gameIDLen) + int(userIDLen) + int(bodyLen)

	buffer := make([]byte, totalLen)
	offset := 0

	// 版本 (1字节)
	buffer[offset] = 1
	offset++

	// 类型 (2字节)
	binary.BigEndian.PutUint16(buffer[offset:offset+2], msgType)
	offset += 2

	// 标志 (1字节)
	buffer[offset] = 0
	offset++

	// 序列号 (4字节)
	binary.BigEndian.PutUint32(buffer[offset:offset+4], 1)
	offset += 4

	// 时间戳 (8字节)
	binary.BigEndian.PutUint64(buffer[offset:offset+8], uint64(time.Now().Unix()))
	offset += 8

	// 消息体长度 (4字节)
	binary.BigEndian.PutUint32(buffer[offset:offset+4], bodyLen)
	offset += 4

	// 校验和 (4字节) - 跳过，稍后填充
	checksumOffset := offset
	offset += 4

	// 游戏ID长度 (2字节)
	binary.BigEndian.PutUint16(buffer[offset:offset+2], gameIDLen)
	offset += 2

	// 用户ID长度 (2字节)
	binary.BigEndian.PutUint16(buffer[offset:offset+2], userIDLen)
	offset += 2

	// 游戏ID
	copy(buffer[offset:offset+int(gameIDLen)], gameIDBytes)
	offset += int(gameIDLen)

	// 用户ID
	copy(buffer[offset:offset+int(userIDLen)], userIDBytes)
	offset += int(userIDLen)

	// 消息体
	copy(buffer[offset:], body)

	// 计算校验和（与BinaryCodec.Encode完全一致）
	// 校验和计算所有数据，除了校验和字段本身
	checksumData := make([]byte, 0, len(buffer)-4)
	checksumData = append(checksumData, buffer[:checksumOffset]...)   // 校验和字段之前的所有数据
	checksumData = append(checksumData, buffer[checksumOffset+4:]...) // 校验和字段之后的所有数据
	checksum := crc32.ChecksumIEEE(checksumData)

	// 写入校验和
	binary.BigEndian.PutUint32(buffer[checksumOffset:checksumOffset+4], checksum)

	return buffer
}
