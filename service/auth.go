package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	deverjwt "github.com/shemic/dever/auth/jwt"
	"github.com/shemic/dever/config"
	"github.com/shemic/dever/orm"

	usermodel "my/package/user/model"
)

const (
	accessTokenTTL  = 7 * 24 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

type AuthService struct{}

type RegisterRequest struct {
	Account  string
	Password string
	Name     string
}

type LoginRequest struct {
	Account  string
	Password string
}

func (AuthService) Register(ctx context.Context, req RegisterRequest) (map[string]any, error) {
	account := normalizeAccount(req.Account)
	password := strings.TrimSpace(req.Password)
	name := strings.TrimSpace(req.Name)
	if account == "" || password == "" {
		return nil, fmt.Errorf("账号和密码不能为空")
	}
	if len([]rune(password)) < 6 {
		return nil, fmt.Errorf("密码不能少于 6 位")
	}
	credentialModel := usermodel.NewCredentialModel()
	if credentialModel.Find(ctx, map[string]any{
		"provider": usermodel.CredentialProviderPassword,
		"account":  account,
	}) != nil {
		return nil, fmt.Errorf("账号已存在")
	}
	if name == "" {
		name = account
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var userID uint64
	if err := orm.Transaction(ctx, func(tx context.Context) error {
		userID = uint64(usermodel.NewUserModel().Insert(tx, map[string]any{
			"account":    account,
			"name":       name,
			"mobile":     "",
			"status":     usermodel.UserStatusEnabled,
			"remark":     "",
			"created_at": now,
		}))
		if userID == 0 {
			return fmt.Errorf("注册失败")
		}
		credentialID := uint64(credentialModel.Insert(tx, map[string]any{
			"user_id":       userID,
			"provider":      usermodel.CredentialProviderPassword,
			"account":       account,
			"password_hash": passwordHash,
			"status":        usermodel.CredentialStatusEnabled,
			"created_at":    now,
		}))
		if credentialID == 0 {
			return fmt.Errorf("注册失败")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return authPayload(ctx, usermodel.NewUserModel().Find(ctx, map[string]any{"id": userID}), account)
}

func (AuthService) Login(ctx context.Context, req LoginRequest) (map[string]any, error) {
	account := normalizeAccount(req.Account)
	password := strings.TrimSpace(req.Password)
	if account == "" || password == "" {
		return nil, fmt.Errorf("账号和密码不能为空")
	}

	credential := usermodel.NewCredentialModel().Find(ctx, map[string]any{
		"provider": usermodel.CredentialProviderPassword,
		"account":  account,
		"status":   usermodel.CredentialStatusEnabled,
	})
	if credential == nil || !verifyPassword(password, credential.PasswordHash) {
		return nil, fmt.Errorf("账号或密码错误")
	}

	user := usermodel.NewUserModel().Find(ctx, map[string]any{
		"id":     credential.UserID,
		"status": usermodel.UserStatusEnabled,
	})
	if user == nil {
		return nil, fmt.Errorf("账号或密码错误")
	}
	return authPayload(ctx, user, account)
}

func (AuthService) Profile(ctx context.Context) (map[string]any, error) {
	user, err := CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"user": userPayload(ctx, *user, "")}, nil
}

func (AuthService) Refresh(ctx context.Context, refreshToken string) (map[string]any, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("刷新Token不能为空")
	}

	tokenModel := usermodel.NewTokenModel()
	tokenRow := tokenModel.Find(ctx, map[string]any{
		"type":       usermodel.TokenTypeRefresh,
		"token_hash": hashSecret(refreshToken),
		"status":     usermodel.TokenStatusEnabled,
	})
	if tokenRow == nil || !tokenRow.ExpiresAt.After(time.Now()) {
		return nil, NewAuthRequiredError("刷新Token无效")
	}

	user := usermodel.NewUserModel().Find(ctx, map[string]any{
		"id":     tokenRow.UserID,
		"status": usermodel.UserStatusEnabled,
	})
	if user == nil {
		return nil, NewAuthRequiredError("用户不存在或已停用")
	}
	tokenModel.Update(ctx, map[string]any{"id": tokenRow.ID}, map[string]any{
		"used_at": time.Now(),
	})
	return authPayload(ctx, user, "")
}

func (AuthService) Logout(ctx context.Context, refreshToken string) (map[string]any, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return map[string]any{"ok": true}, nil
	}
	usermodel.NewTokenModel().Update(ctx, map[string]any{
		"type":       usermodel.TokenTypeRefresh,
		"token_hash": hashSecret(refreshToken),
		"status":     usermodel.TokenStatusEnabled,
	}, map[string]any{
		"status":  usermodel.TokenStatusRevoked,
		"used_at": time.Now(),
	})
	return map[string]any{"ok": true}, nil
}

func authPayload(ctx context.Context, user *usermodel.User, account string) (map[string]any, error) {
	if user == nil {
		return nil, fmt.Errorf("用户不存在")
	}
	expiredAt := time.Now().Add(accessTokenTTL)
	token, err := createUserToken(user.ID, expiredAt)
	if err != nil {
		return nil, err
	}
	refreshToken, err := createRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"token":         token,
		"refresh_token": refreshToken,
		"user":          userPayload(ctx, *user, account),
		"exp":           expiredAt.UnixMilli(),
	}, nil
}

func createUserToken(userID uint64, expiredAt time.Time) (string, error) {
	cfg, err := config.Load("")
	if err != nil {
		return "", fmt.Errorf("读取配置失败")
	}
	signer, err := deverjwt.ResolveSigner(cfg.Auth, "user", "default")
	if err != nil {
		return "", fmt.Errorf("JWT密钥未配置")
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":        fmt.Sprintf("%d", userID),
		"actor_id":   fmt.Sprintf("%d", userID),
		"actor_type": ActorTypeUser,
		"user_id":    fmt.Sprintf("%d", userID),
		"site":       TokenScopeUser,
		"scope":      TokenScopeUser,
		"exp":        expiredAt.Unix(),
		"iat":        now.Unix(),
	})
	return token.SignedString([]byte(signer.Secret))
}

func createRefreshToken(ctx context.Context, userID uint64) (string, error) {
	token, err := newRefreshToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	usermodel.NewTokenModel().Insert(ctx, map[string]any{
		"user_id":    userID,
		"type":       usermodel.TokenTypeRefresh,
		"token_hash": hashSecret(token),
		"status":     usermodel.TokenStatusEnabled,
		"expires_at": now.Add(refreshTokenTTL),
		"created_at": now,
	})
	return token, nil
}

func userPayload(ctx context.Context, user usermodel.User, account string) map[string]any {
	if account == "" {
		account = primaryAccount(ctx, user.ID)
	}
	return map[string]any{
		"id":         user.ID,
		"name":       user.Name,
		"mobile":     user.Mobile,
		"account":    firstNonEmpty(user.Account, account),
		"status":     user.Status,
		"created_at": user.CreatedAt,
	}
}

func primaryAccount(ctx context.Context, userID uint64) string {
	if userID == 0 {
		return ""
	}
	credential := usermodel.NewCredentialModel().Find(ctx, map[string]any{
		"user_id":  userID,
		"provider": usermodel.CredentialProviderPassword,
		"status":   usermodel.CredentialStatusEnabled,
	})
	if credential == nil {
		return ""
	}
	return credential.Account
}

func normalizeAccount(account string) string {
	return strings.ToLower(strings.TrimSpace(account))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

func encodeScopes(scopes []string) string {
	cleaned := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if item := strings.TrimSpace(scope); item != "" {
			cleaned = append(cleaned, item)
		}
	}
	if len(cleaned) == 0 {
		cleaned = []string{TokenScopeUser}
	}
	payload, _ := json.Marshal(cleaned)
	return string(payload)
}
