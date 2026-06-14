package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	refreshTokenByteSize = 32
	apiKeyByteSize       = 32
	apiKeyPrefix         = "uapi"
)

func hashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("密码不能为空")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成密码哈希失败")
	}
	return string(hashed), nil
}

func verifyPassword(password string, hashed string) bool {
	password = strings.TrimSpace(password)
	hashed = strings.TrimSpace(hashed)
	if password == "" || hashed == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}

func newRefreshToken() (string, error) {
	return randomToken(refreshTokenByteSize)
}

func newAPIKey() (string, error) {
	token, err := randomToken(apiKeyByteSize)
	if err != nil {
		return "", err
	}
	return apiKeyPrefix + "_" + token, nil
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		size = 32
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机密钥失败")
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func visiblePrefix(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) <= 12 {
		return secret
	}
	return secret[:12]
}
