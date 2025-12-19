package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"time"

	"datamiddleware/pkg/types"
)

// Codec 编解码器接口
type Codec interface {
	Encode(msg *types.Message) ([]byte, error)
	Decode(data []byte) (msg *types.Message, consumed int, err error)
}

// DecodeResult 解码结果
type DecodeResult struct {
	Message  *types.Message // 解析出的消息，如果为nil表示数据不足
	Consumed int            // 消耗的字节数
}

// JSONCodec JSON编解码器
type JSONCodec struct{}

// NewJSONCodec 创建JSON编解码器
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// Encode 编码消息
func (c *JSONCodec) Encode(msg *types.Message) ([]byte, error) {
	// 序列化消息头
	headerData, err := json.Marshal(msg.Header)
	if err != nil {
		return nil, fmt.Errorf("序列化消息头失败: %w", err)
	}

	// 计算校验和
	checksum := crc32.ChecksumIEEE(headerData)
	checksum = crc32.Update(checksum, crc32.IEEETable, msg.Body)

	// 更新校验和
	msg.Header.Checksum = checksum
	msg.Header.BodyLength = uint32(len(msg.Body))
	msg.Header.Timestamp = time.Now().Unix()

	// 重新序列化消息头（包含校验和）
	headerData, err = json.Marshal(msg.Header)
	if err != nil {
		return nil, fmt.Errorf("序列化消息头失败: %w", err)
	}

	// 构造完整消息
	// 格式: [消息头长度(4字节)] + [消息头数据] + [消息体数据]
	headerLen := uint32(len(headerData))
	buffer := make([]byte, 4+headerLen+uint32(len(msg.Body)))

	// 写入消息头长度
	binary.BigEndian.PutUint32(buffer[0:4], headerLen)

	// 写入消息头数据
	copy(buffer[4:4+headerLen], headerData)

	// 写入消息体数据
	copy(buffer[4+headerLen:], msg.Body)

	return buffer, nil
}

// Decode 解码消息
func (c *JSONCodec) Decode(data []byte) (msg *types.Message, consumed int, err error) {
	if len(data) < 4 {
		return nil, 0, fmt.Errorf("数据长度不足，无法解析消息头长度")
	}

	// 读取消息头长度
	headerLen := binary.BigEndian.Uint32(data[0:4])
	if len(data) < int(4+headerLen) {
		return nil, 4, fmt.Errorf("数据长度不足，无法解析完整消息头")
	}

	// 读取消息头数据
	headerData := data[4 : 4+headerLen]

	// 反序列化消息头
	var header types.MessageHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		return nil, int(4 + headerLen), fmt.Errorf("反序列化消息头失败: %w", err)
	}

	// 验证消息头长度
	expectedTotalLen := 4 + headerLen + header.BodyLength
	if uint32(len(data)) < expectedTotalLen {
		return nil, int(4 + headerLen), fmt.Errorf("消息长度不足，期望%d，实际%d", expectedTotalLen, len(data))
	}

	// 读取消息体数据
	bodyData := data[4+headerLen : expectedTotalLen]

	// 验证校验和
	checksum := crc32.ChecksumIEEE(headerData)
	checksum = crc32.Update(checksum, crc32.IEEETable, bodyData)
	if checksum != header.Checksum {
		return nil, int(expectedTotalLen), fmt.Errorf("校验和验证失败，期望0x%x，实际0x%x", header.Checksum, checksum)
	}

	return &types.Message{
		Header: header,
		Body:   bodyData,
	}, int(expectedTotalLen), nil
}

// BinaryCodec 二进制编解码器（性能优化版本）
type BinaryCodec struct{}

// NewBinaryCodec 创建二进制编解码器
func NewBinaryCodec() *BinaryCodec {
	return &BinaryCodec{}
}

// Encode 编码消息（二进制格式）
// 格式: [版本(1)] [类型(2)] [标志(1)] [序列号(4)] [时间戳(8)] [体长度(4)] [校验和(4)] [游戏ID长度(2)] [游戏ID] [用户ID长度(2)] [用户ID] [消息体]
func (c *BinaryCodec) Encode(msg *types.Message) ([]byte, error) {
	// 准备字符串数据
	gameIDBytes := []byte(msg.Header.GameID)
	userIDBytes := []byte(msg.Header.UserID)

	// 计算消息总长度
	gameIDLen := uint16(len(gameIDBytes))
	userIDLen := uint16(len(userIDBytes))
	bodyLen := uint32(len(msg.Body))

	// 固定头部长度: 版本(1) + 类型(2) + 标志(1) + 序列号(4) + 时间戳(8) + 体长度(4) + 校验和(4) + 游戏ID长度(2) + 用户ID长度(2)
	fixedHeaderLen := 1 + 2 + 1 + 4 + 8 + 4 + 4 + 2 + 2
	totalLen := fixedHeaderLen + int(gameIDLen) + int(userIDLen) + int(bodyLen)

	buffer := make([]byte, totalLen)
	offset := 0

	// 版本
	buffer[offset] = msg.Header.Version
	offset++

	// 类型
	binary.BigEndian.PutUint16(buffer[offset:offset+2], uint16(msg.Header.Type))
	offset += 2

	// 标志
	buffer[offset] = byte(msg.Header.Flags)
	offset++

	// 序列号
	binary.BigEndian.PutUint32(buffer[offset:offset+4], msg.Header.SequenceID)
	offset += 4

	// 时间戳
	// 注意：这里不重新设置时间戳，使用消息头中的时间戳
	binary.BigEndian.PutUint64(buffer[offset:offset+8], uint64(msg.Header.Timestamp))
	offset += 8

	// 消息体长度
	binary.BigEndian.PutUint32(buffer[offset:offset+4], bodyLen)
	offset += 4

	// 跳过校验和字段（4字节），稍后填充
	checksumOffset := offset
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

	// 消息体
	copy(buffer[offset:], msg.Body)

	// 计算校验和（所有数据，除了校验和字段）
	checksumData := make([]byte, 0, len(buffer)-4)
	checksumData = append(checksumData, buffer[:checksumOffset]...)   // 校验和字段之前的所有数据
	checksumData = append(checksumData, buffer[checksumOffset+4:]...) // 校验和字段之后的所有数据
	checksum := crc32.ChecksumIEEE(checksumData)

	// 写入校验和
	binary.BigEndian.PutUint32(buffer[checksumOffset:checksumOffset+4], checksum)
	msg.Header.Checksum = checksum

	return buffer, nil
}

// Decode 解码消息（二进制格式）
func (c *BinaryCodec) Decode(data []byte) (msg *types.Message, consumed int, err error) {
	if len(data) < 28 { // 最小消息长度
		return nil, 0, fmt.Errorf("数据长度不足，无法解析消息")
	}

	offset := 0
	header := types.MessageHeader{}

	// 版本
	header.Version = data[offset]
	offset++

	// 类型
	header.Type = types.MessageType(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2

	// 标志
	header.Flags = types.MessageFlag(data[offset])
	offset++

	// 序列号
	header.SequenceID = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 时间戳
	header.Timestamp = int64(binary.BigEndian.Uint64(data[offset : offset+8]))
	offset += 8

	// 消息体长度
	header.BodyLength = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 校验和
	header.Checksum = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 游戏ID长度
	gameIDLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// 用户ID长度
	userIDLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// 游戏ID
	if len(data) < offset+int(gameIDLen) {
		return nil, 28, fmt.Errorf("数据长度不足，无法解析游戏ID")
	}
	header.GameID = string(data[offset : offset+int(gameIDLen)])
	offset += int(gameIDLen)

	// 用户ID
	if len(data) < offset+int(userIDLen) {
		return nil, offset, fmt.Errorf("数据长度不足，无法解析用户ID")
	}
	header.UserID = string(data[offset : offset+int(userIDLen)])
	offset += int(userIDLen)

	// 消息体
	if len(data) < offset+int(header.BodyLength) {
		return nil, offset, fmt.Errorf("数据长度不足，无法解析消息体")
	}
	body := data[offset : offset+int(header.BodyLength)]

	// 验证校验和（所有数据，除了校验和字段）
	checksumData := make([]byte, 0, len(data)-4)
	checksumOffset := 20                                            // 校验和字段的起始位置
	checksumData = append(checksumData, data[:checksumOffset]...)   // 校验和字段之前的所有数据
	checksumData = append(checksumData, data[checksumOffset+4:]...) // 校验和字段之后的所有数据
	expectedChecksum := crc32.ChecksumIEEE(checksumData)
	if expectedChecksum != header.Checksum {
		return nil, offset + int(header.BodyLength), fmt.Errorf("校验和验证失败，期望0x%x，实际0x%x", header.Checksum, expectedChecksum)
	}

	totalConsumed := offset + int(header.BodyLength)
	return &types.Message{
		Header: header,
		Body:   body,
	}, totalConsumed, nil
}

// CreateHeartbeatMessage 创建心跳消息
func CreateHeartbeatMessage(sequenceID uint32) *types.Message {
	return &types.Message{
		Header: types.MessageHeader{
			Version:    types.ProtocolVersion,
			Type:       types.MessageTypeHeartbeat,
			Flags:      types.FlagNone,
			SequenceID: sequenceID,
			Timestamp:  time.Now().Unix(),
		},
		Body: []byte{},
	}
}

// CreateHandshakeMessage 创建握手消息
func CreateHandshakeMessage(gameID, userID string, sequenceID uint32) *types.Message {
	body := map[string]interface{}{
		"game_id": gameID,
		"user_id": userID,
	}
	bodyData, _ := json.Marshal(body)

	return &types.Message{
		Header: types.MessageHeader{
			Version:    types.ProtocolVersion,
			Type:       types.MessageTypeHandshake,
			Flags:      types.FlagNone,
			SequenceID: sequenceID,
			GameID:     gameID,
			UserID:     userID,
			Timestamp:  time.Now().Unix(),
			BodyLength: uint32(len(bodyData)),
		},
		Body: bodyData,
	}
}

// CreateErrorMessage 创建错误消息
func CreateErrorMessage(code int, message string, sequenceID uint32) *types.Message {
	body := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	bodyData, _ := json.Marshal(body)

	return &types.Message{
		Header: types.MessageHeader{
			Version:    types.ProtocolVersion,
			Type:       types.MessageTypeError,
			Flags:      types.FlagNone,
			SequenceID: sequenceID,
			Timestamp:  time.Now().Unix(),
			BodyLength: uint32(len(bodyData)),
		},
		Body: bodyData,
	}
}
