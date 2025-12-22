// Package main 简单的TCP客户端调试工具
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"time"

	"datamiddleware/internal/common/types"
)

func createSimpleHeartbeatMessage() ([]byte, error) {
	header := types.MessageHeader{
		Version:    types.ProtocolVersion,
		Type:       types.MessageTypeHeartbeat,
		Flags:      types.FlagNeedResponse, // 需要响应
		SequenceID: 1,
		GameID:     "game1",
		UserID:     "test",
		Timestamp:  time.Now().Unix(),
		BodyLength: 0,
	}

	// 准备字符串数据
	gameIDBytes := []byte(header.GameID)
	userIDBytes := []byte(header.UserID)

	// 计算消息总长度
	gameIDLen := uint16(len(gameIDBytes))
	userIDLen := uint16(len(userIDBytes))
	bodyLen := uint32(len([]byte{}))

	// 固定头部长度: 版本(1) + 类型(2) + 标志(1) + 序列号(4) + 时间戳(8) + 体长度(4) + 校验和(4) + 游戏ID长度(2) + 用户ID长度(2)
	fixedHeaderLen := 1 + 2 + 1 + 4 + 8 + 4 + 4 + 2 + 2
	totalLen := fixedHeaderLen + int(gameIDLen) + int(userIDLen) + int(bodyLen)

	buffer := make([]byte, totalLen)
	offset := 0

	// 版本
	buffer[offset] = header.Version
	offset++

	// 类型
	binary.BigEndian.PutUint16(buffer[offset:offset+2], uint16(header.Type))
	offset += 2

	// 标志
	buffer[offset] = byte(header.Flags)
	offset++

	// 序列号
	binary.BigEndian.PutUint32(buffer[offset:offset+4], header.SequenceID)
	offset += 4

	// 时间戳
	binary.BigEndian.PutUint64(buffer[offset:offset+8], uint64(header.Timestamp))
	offset += 8

	// 消息体长度
	binary.BigEndian.PutUint32(buffer[offset:offset+4], bodyLen)
	offset += 4

	// 计算校验和 (暂时设为0，稍后计算)
	checksumOffset := offset
	binary.BigEndian.PutUint32(buffer[offset:offset+4], 0)
	offset += 4

	// 游戏ID长度
	binary.BigEndian.PutUint16(buffer[offset:offset+2], gameIDLen)
	offset += 2

	// 用户ID长度
	binary.BigEndian.PutUint16(buffer[offset:offset+2], userIDLen)
	offset += 2

	// 游戏ID
	copy(buffer[offset:offset+int(gameIDLen)], gameIDBytes)
	offset += int(gameIDLen)

	// 用户ID
	copy(buffer[offset:offset+int(userIDLen)], userIDBytes)
	offset += int(userIDLen)

	// 消息体 (空)

	// 计算校验和 (对整个消息进行CRC32)
	checksum := crc32.ChecksumIEEE(buffer[:totalLen])
	binary.BigEndian.PutUint32(buffer[checksumOffset:checksumOffset+4], checksum)

	fmt.Printf("发送消息详情 (二进制协议):\n")
	fmt.Printf("  消息类型: %d (心跳)\n", header.Type)
	fmt.Printf("  序列号: %d\n", header.SequenceID)
	fmt.Printf("  游戏ID: %s\n", header.GameID)
	fmt.Printf("  用户ID: %s\n", header.UserID)
	fmt.Printf("  时间戳: %d\n", header.Timestamp)
	fmt.Printf("  校验和: %d (0x%x)\n", checksum, checksum)
	fmt.Printf("  总长度: %d 字节\n", len(buffer))

	// 打印十六进制数据用于调试
	fmt.Printf("  消息数据 (十六进制): %x\n", buffer)

	return buffer, nil
}

func main() {
	fmt.Println("=== DataMiddleware TCP客户端调试工具 ===")

	// 连接到TCP服务器
	conn, err := net.Dial("tcp", "localhost:9090")
	if err != nil {
		log.Fatalf("连接TCP服务器失败: %v", err)
	}
	defer conn.Close()

	fmt.Println("✅ TCP连接成功")

	// 创建心跳消息
	message, err := createSimpleHeartbeatMessage()
	if err != nil {
		log.Fatalf("创建消息失败: %v", err)
	}

	// 发送消息
	fmt.Println("\n📤 发送心跳消息...")
	_, err = conn.Write(message)
	if err != nil {
		log.Fatalf("发送消息失败: %v", err)
	}

	fmt.Println("✅ 消息发送成功")

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 尝试读取响应
	fmt.Println("\n📥 等待服务器响应...")
	responseBuffer := make([]byte, 1024)
	n, err := conn.Read(responseBuffer)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 收到响应: %d 字节\n", n)
	fmt.Printf("响应数据 (十六进制): %x\n", responseBuffer[:n])

	// 尝试解析响应
	if n >= 4 {
		headerLen := binary.BigEndian.Uint32(responseBuffer[0:4])
		fmt.Printf("响应消息头长度: %d\n", headerLen)

		if n >= int(4+headerLen) {
			headerData := responseBuffer[4 : 4+headerLen]
			fmt.Printf("响应消息头JSON: %s\n", string(headerData))
		}
	}
}
