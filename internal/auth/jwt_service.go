package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// JWTClaims JWT声明
type JWTClaims struct {
	UserID   string `json:"user_id"`
	GameID   string `json:"game_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTService JWT认证服务
type JWTService struct {
	secretKey     []byte
	expireTime    time.Duration
	refreshExpire time.Duration
	logger        logger.Logger
}

// NewJWTService 创建JWT服务
func NewJWTService(config types.JWTConfig, log logger.Logger) *JWTService {
	return &JWTService{
		secretKey:     []byte(config.Secret),
		expireTime:    time.Duration(config.Expire) * time.Second,
		refreshExpire: 7 * 24 * time.Hour, // 7天刷新过期时间
		logger:        log,
	}
}

// GenerateToken 生成JWT令牌
func (s *JWTService) GenerateToken(userID, gameID, username string) (*types.TokenPair, error) {
	now := time.Now()

	// 生成访问令牌
	accessClaims := JWTClaims{
		UserID:   userID,
		GameID:   gameID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "datamiddleware",
			Subject:   userID,
			Audience:  jwt.ClaimStrings{gameID},
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expireTime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        s.generateJTI(),
		},
	}

	accessToken, err := s.signToken(accessClaims)
	if err != nil {
		s.logger.Error("生成访问令牌失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("生成访问令牌失败: %w", err)
	}

	// 生成刷新令牌
	refreshClaims := JWTClaims{
		UserID:   userID,
		GameID:   gameID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "datamiddleware",
			Subject:   userID,
			Audience:  jwt.ClaimStrings{gameID},
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshExpire)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        s.generateJTI(),
		},
	}

	refreshToken, err := s.signToken(refreshClaims)
	if err != nil {
		s.logger.Error("生成刷新令牌失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	tokenPair := &types.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.expireTime.Seconds()),
		ExpiresAt:    now.Add(s.expireTime).Unix(),
	}

	s.logger.Info("JWT令牌生成成功", "user_id", userID, "expires_at", tokenPair.ExpiresAt)
	return tokenPair, nil
}

// ValidateToken 验证JWT令牌
func (s *JWTService) ValidateToken(tokenString string) (*types.TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		s.logger.Warn("JWT令牌解析失败", "error", err)
		return nil, fmt.Errorf("令牌无效: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		tokenClaims := &types.TokenClaims{
			UserID:   claims.UserID,
			GameID:   claims.GameID,
			Username: claims.Username,
			ExpiresAt: claims.ExpiresAt.Time.Unix(),
			IssuedAt:  claims.IssuedAt.Time.Unix(),
			TokenID:   claims.ID,
		}

		s.logger.Debug("JWT令牌验证成功", "user_id", claims.UserID, "token_id", claims.ID)
		return tokenClaims, nil
	}

	s.logger.Warn("JWT令牌声明无效")
	return nil, fmt.Errorf("令牌声明无效")
}

// RefreshToken 刷新访问令牌
func (s *JWTService) RefreshToken(refreshTokenString string) (*types.TokenPair, error) {
	// 验证刷新令牌
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("刷新令牌无效: %w", err)
	}

	// 生成新的令牌对
	tokenPair, err := s.GenerateToken(claims.UserID, claims.GameID, claims.Username)
	if err != nil {
		return nil, fmt.Errorf("生成新令牌失败: %w", err)
	}

	s.logger.Info("JWT令牌刷新成功", "user_id", claims.UserID, "old_token_id", claims.TokenID)
	return tokenPair, nil
}

// RevokeToken 撤销令牌（添加到黑名单）
func (s *JWTService) RevokeToken(tokenID string) error {
	// TODO: 实现令牌黑名单机制
	// 这里可以添加到Redis或数据库中
	s.logger.Info("JWT令牌已撤销", "token_id", tokenID)
	return nil
}

// IsTokenRevoked 检查令牌是否已撤销
func (s *JWTService) IsTokenRevoked(tokenID string) bool {
	// TODO: 检查令牌黑名单
	return false
}

// ExtractTokenFromHeader 从Authorization头提取令牌
func (s *JWTService) ExtractTokenFromHeader(authHeader string) (string, error) {
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return "", fmt.Errorf("无效的Authorization头格式")
	}
	return authHeader[7:], nil
}

// GetTokenExpiration 获取令牌过期时间
func (s *JWTService) GetTokenExpiration(tokenString string) (time.Time, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.secretKey, nil
	})

	if err != nil {
		return time.Time{}, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok {
		return claims.ExpiresAt.Time, nil
	}

	return time.Time{}, fmt.Errorf("无法获取令牌过期时间")
}

// IsTokenExpired 检查令牌是否过期
func (s *JWTService) IsTokenExpired(tokenString string) bool {
	expiration, err := s.GetTokenExpiration(tokenString)
	if err != nil {
		return true
	}
	return time.Now().After(expiration)
}

// GenerateAPIKey 生成API密钥
func (s *JWTService) GenerateAPIKey(gameID string) (*types.APIKey, error) {
	// 生成API密钥
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("生成API密钥失败: %w", err)
	}
	apiKey := hex.EncodeToString(keyBytes)

	// 生成密钥ID
	keyIDBytes := make([]byte, 8)
	if _, err := rand.Read(keyIDBytes); err != nil {
		return nil, fmt.Errorf("生成密钥ID失败: %w", err)
	}
	keyID := hex.EncodeToString(keyIDBytes)

	apiKeyObj := &types.APIKey{
		KeyID:     keyID,
		Key:       apiKey,
		GameID:    gameID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1年过期
		IsActive:  true,
	}

	s.logger.Info("API密钥生成成功", "key_id", keyID, "game_id", gameID)
	return apiKeyObj, nil
}

// ValidateAPIKey 验证API密钥
func (s *JWTService) ValidateAPIKey(apiKey string) (*types.APIKey, error) {
	// TODO: 从数据库或缓存中验证API密钥
	// 这里暂时返回模拟数据
	s.logger.Debug("API密钥验证", "key_prefix", apiKey[:8]+"...")
	return nil, fmt.Errorf("API密钥验证未实现")
}

// Helper methods

func (s *JWTService) signToken(claims JWTClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *JWTService) generateJTI() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机生成失败，使用时间戳
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
