package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/shemic/dever/orm"

	usermodel "github.com/dever-package/user/model"
)

type ExternalLoginRequest struct {
	Provider string
	Subject  string
	Account  string
	Name     string
	Mobile   string
}

func (AuthService) ExternalLogin(ctx context.Context, req ExternalLoginRequest) (map[string]any, error) {
	request, err := normalizeExternalLoginRequest(req)
	if err != nil {
		return nil, err
	}

	credential := findExternalCredential(ctx, request.Provider, request.Subject)
	if credential != nil {
		return loginWithExternalCredential(ctx, credential)
	}

	userID, err := createExternalUser(ctx, request)
	if err != nil {
		// The provider/account unique index is the concurrency guard. If another
		// request created the same identity first, reuse that completed record.
		credential = findExternalCredential(ctx, request.Provider, request.Subject)
		if credential != nil {
			return loginWithExternalCredential(ctx, credential)
		}
		return nil, err
	}
	user := usermodel.NewUserModel().Find(ctx, map[string]any{"id": userID})
	return authPayload(ctx, user, request.Account)
}

func normalizeExternalLoginRequest(req ExternalLoginRequest) (ExternalLoginRequest, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	subject := strings.TrimSpace(req.Subject)
	if provider == "" || provider == usermodel.CredentialProviderPassword || subject == "" {
		return ExternalLoginRequest{}, fmt.Errorf("外部登录身份无效")
	}
	if len([]rune(provider)) > 32 || len([]rune(subject)) > 128 {
		return ExternalLoginRequest{}, fmt.Errorf("外部登录身份过长")
	}
	for index, char := range provider {
		if index == 0 && char == '-' {
			return ExternalLoginRequest{}, fmt.Errorf("外部登录类型无效")
		}
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return ExternalLoginRequest{}, fmt.Errorf("外部登录类型无效")
	}

	account := normalizeAccount(req.Account)
	if account == "" || len([]rune(account)) > 128 {
		account = externalFallbackAccount(provider, subject)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "外部用户"
	}
	return ExternalLoginRequest{
		Provider: provider,
		Subject:  subject,
		Account:  account,
		Name:     truncateRunes(name, 64),
		Mobile:   truncateRunes(strings.TrimSpace(req.Mobile), 32),
	}, nil
}

func findExternalCredential(ctx context.Context, provider string, subject string) *usermodel.Credential {
	return usermodel.NewCredentialModel().Find(ctx, map[string]any{
		"provider": provider,
		"account":  subject,
	})
}

func loginWithExternalCredential(ctx context.Context, credential *usermodel.Credential) (map[string]any, error) {
	if credential == nil || credential.Status != usermodel.CredentialStatusEnabled {
		return nil, fmt.Errorf("外部登录凭据不存在或已停用")
	}
	user := usermodel.NewUserModel().Find(ctx, map[string]any{
		"id":     credential.UserID,
		"status": usermodel.UserStatusEnabled,
	})
	if user == nil {
		return nil, fmt.Errorf("用户不存在或已停用")
	}
	return authPayload(ctx, user, user.Account)
}

func createExternalUser(ctx context.Context, req ExternalLoginRequest) (userID uint64, resultErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok {
				resultErr = recoveredErr
			} else {
				resultErr = fmt.Errorf("%v", recovered)
			}
			userID = 0
		}
	}()

	now := time.Now()
	resultErr = orm.Transaction(ctx, func(tx context.Context) error {
		userID = uint64(usermodel.NewUserModel().Insert(tx, map[string]any{
			"account":         req.Account,
			"name":            req.Name,
			"mobile":          req.Mobile,
			"avatar_file_id":  uint64(0),
			"session_version": uint64(1),
			"status":          usermodel.UserStatusEnabled,
			"remark":          "",
			"created_at":      now,
		}))
		if userID == 0 {
			return fmt.Errorf("创建用户失败")
		}
		credentialID := usermodel.NewCredentialModel().Insert(tx, map[string]any{
			"user_id":       userID,
			"provider":      req.Provider,
			"account":       req.Subject,
			"password_hash": "",
			"status":        usermodel.CredentialStatusEnabled,
			"created_at":    now,
		})
		if credentialID == 0 {
			return fmt.Errorf("创建外部登录凭据失败")
		}
		return initializeRegistrationBenefits(tx, userID, map[string]any{
			"id":     userID,
			"name":   req.Name,
			"mobile": req.Mobile,
		}, now)
	})
	return userID, resultErr
}

func externalFallbackAccount(provider string, subject string) string {
	digest := sha256.Sum256([]byte(subject))
	return fmt.Sprintf("%s_%x", provider, digest[:8])
}

func truncateRunes(value string, limit int) string {
	chars := []rune(value)
	if limit <= 0 || len(chars) <= limit {
		return value
	}
	return string(chars[:limit])
}
