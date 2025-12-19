package test

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"datamiddleware/internal/protocol"
	"datamiddleware/pkg/types"
)

// TestTCPClient 测试TCP客户端
func TestTCPClient(t *testing.T) {
	// 连接到服务器
	conn, err := net.Dial("tcp", "127.0.0.1:9090")
	if err != nil {
		t.Skip("TCP服务器未运行，跳过测试")
		return
	}
	defer conn.Close()

	codec := protocol.NewBinaryCodec()

	// 测试编解码一致性
	t.Run("CodecConsistency", func(t *testing.T) {
		heartbeatMsg := protocol.CreateHeartbeatMessage(1)

		// 编码消息
		data, err := codec.Encode(heartbeatMsg)
		if err != nil {
			t.Fatalf("编码心跳消息失败: %v", err)
		}

		t.Logf("编码后消息长度: %d", len(data))
		t.Logf("编码后消息数据: %x", data)

		// 解析各个字段
		t.Logf("版本: %x", data[0])
		t.Logf("类型: %x", data[1:3])
		t.Logf("标志: %x", data[3])
		t.Logf("序列号: %x", data[4:8])
		t.Logf("时间戳: %x", data[8:16])
		t.Logf("体长度: %x", data[16:20])
		t.Logf("校验和: %x", data[20:24])
		t.Logf("游戏ID长度: %x", data[24:26])
		t.Logf("用户ID长度: %x", data[26:28])

		// 立即解码验证一致性
		decoded, consumed, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("解码心跳消息失败: %v", err)
		}

		t.Logf("解码消耗字节数: %d", consumed)
		t.Logf("解码后消息类型: %v", decoded.Header.Type)
		t.Logf("解码后序列号: %d", decoded.Header.SequenceID)

		if decoded.Header.Type != types.MessageTypeHeartbeat {
			t.Errorf("期望心跳消息，实际得到 %v", decoded.Header.Type)
		}
		if decoded.Header.SequenceID != 1 {
			t.Errorf("期望序列号1，实际得到 %d", decoded.Header.SequenceID)
		}
	})

	// 测试心跳（最简单的消息）
	t.Run("SimpleHeartbeat", func(t *testing.T) {
		heartbeatMsg := protocol.CreateHeartbeatMessage(1)

		// 发送心跳消息
		data, err := codec.Encode(heartbeatMsg)
		if err != nil {
			t.Fatalf("编码心跳消息失败: %v", err)
		}

		t.Logf("发送心跳消息，长度: %d", len(data))
		t.Logf("消息数据: %x", data)

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Write(data)
		if err != nil {
			t.Fatalf("发送心跳消息失败: %v", err)
		}

		t.Logf("实际发送字节数: %d", n)

		// 读取响应
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		buffer := make([]byte, 8192)
		n, err = conn.Read(buffer)
		if err != nil {
			t.Fatalf("读取心跳响应失败: %v", err)
		}

		t.Logf("读取到响应，长度: %d", n)

		response, _, err := codec.Decode(buffer[:n])
		if err != nil {
			t.Fatalf("解码心跳响应失败: %v", err)
		}

		if response.Header.Type != types.MessageTypeHeartbeat {
			t.Errorf("期望心跳响应，实际得到 %v", response.Header.Type)
		}

		t.Logf("心跳成功: %+v", response.Header)
	})

	// 测试握手
	t.Run("Handshake", func(t *testing.T) {
		handshakeMsg := protocol.CreateHandshakeMessage("game1", "user123", 1)

		// 发送握手消息
		data, err := codec.Encode(handshakeMsg)
		if err != nil {
			t.Fatalf("编码握手消息失败: %v", err)
		}

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err = conn.Write(data)
		if err != nil {
			t.Fatalf("发送握手消息失败: %v", err)
		}

		// 读取响应
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		buffer := make([]byte, 8192)
		n, err := conn.Read(buffer)
		if err != nil {
			t.Fatalf("读取握手响应失败: %v", err)
		}

		response, _, err := codec.Decode(buffer[:n])
		if err != nil {
			t.Fatalf("解码握手响应失败: %v", err)
		}

		if response.Header.Type != types.MessageTypeHandshake {
			t.Errorf("期望握手响应，实际得到 %v", response.Header.Type)
		}

		t.Logf("握手成功: %+v", response.Header)
	})

	// 测试心跳
	t.Run("Heartbeat", func(t *testing.T) {
		heartbeatMsg := protocol.CreateHeartbeatMessage(2)

		// 发送心跳消息
		data, err := codec.Encode(heartbeatMsg)
		if err != nil {
			t.Fatalf("编码心跳消息失败: %v", err)
		}

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err = conn.Write(data)
		if err != nil {
			t.Fatalf("发送心跳消息失败: %v", err)
		}

		// 读取心跳响应
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		buffer := make([]byte, 8192)
		n, err := conn.Read(buffer)
		if err != nil {
			t.Fatalf("读取心跳响应失败: %v", err)
		}

		response, _, err := codec.Decode(buffer[:n])
		if err != nil {
			t.Fatalf("解码心跳响应失败: %v", err)
		}

		if response.Header.Type != types.MessageTypeHeartbeat {
			t.Errorf("期望心跳响应，实际得到 %v", response.Header.Type)
		}

		t.Logf("心跳成功: %+v", response.Header)
	})

	// 测试玩家登录
	t.Run("PlayerLogin", func(t *testing.T) {
		loginData := map[string]interface{}{
			"username": "testuser",
			"password": "testpass",
		}
		bodyData, _ := json.Marshal(loginData)

		msg := &types.Message{
			Header: types.MessageHeader{
				Version:    types.ProtocolVersion,
				Type:       types.MessageTypePlayerLogin,
				Flags:      types.FlagNone,
				SequenceID: 3,
				GameID:     "game1",
				UserID:     "user123",
				Timestamp:  time.Now().Unix(),
				BodyLength: uint32(len(bodyData)),
			},
			Body: bodyData,
		}

		// 发送登录消息
		data, err := codec.Encode(msg)
		if err != nil {
			t.Fatalf("编码登录消息失败: %v", err)
		}

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err = conn.Write(data)
		if err != nil {
			t.Fatalf("发送登录消息失败: %v", err)
		}

		t.Logf("玩家登录消息已发送")
	})

	// 等待一会儿让服务器处理
	time.Sleep(1 * time.Second)
}

// TestConcurrentClients 测试并发客户端
func TestConcurrentClients(t *testing.T) {
	const numClients = 10

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			conn, err := net.Dial("tcp", "127.0.0.1:9090")
			if err != nil {
				t.Errorf("客户端 %d 连接失败: %v", clientID, err)
				return
			}
			defer conn.Close()

			codec := protocol.NewBinaryCodec()

			// 发送握手
			handshakeMsg := protocol.CreateHandshakeMessage(
				fmt.Sprintf("game%d", clientID%3+1),
				fmt.Sprintf("user%d", clientID),
				uint32(clientID),
			)

			data, err := codec.Encode(handshakeMsg)
			if err != nil {
				t.Errorf("客户端 %d 编码失败: %v", clientID, err)
				return
			}

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err = conn.Write(data)
			if err != nil {
				t.Errorf("客户端 %d 发送失败: %v", clientID, err)
				return
			}

			t.Logf("客户端 %d 握手成功", clientID)

			// 发送一些心跳
			for j := 0; j < 3; j++ {
				time.Sleep(1 * time.Second)
				heartbeatMsg := protocol.CreateHeartbeatMessage(uint32(clientID*10 + j))

				data, err := codec.Encode(heartbeatMsg)
				if err != nil {
					t.Errorf("客户端 %d 编码心跳失败: %v", clientID, err)
					continue
				}

				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_, err = conn.Write(data)
				if err != nil {
					t.Errorf("客户端 %d 发送心跳失败: %v", clientID, err)
					break
				}
			}
		}(i)
	}

	// 等待所有客户端完成
	time.Sleep(5 * time.Second)
}
