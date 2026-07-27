package service

import (
	"context"
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
}

func (AuthService) ExternalLogin(ctx context.Context, req ExternalLoginRequest) (map[string]any, error) {
	request, err := normalizeExternalLoginRequest(req)
	if err != nil {
		return nil, err
	}

	credential := findExternalCredential(ctx, request.Provider, request.Subject)
	if credential != nil {
		if err := syncExternalUserAccount(ctx, credential.UserID, request.Account); err != nil {
			return nil, err
		}
		return loginWithExternalCredential(ctx, credential)
	}
	if request.Account == "" {
		return nil, fmt.Errorf("三方平台未返回手机号，请开通手机号权限后重试")
	}

	userID, err := bindExternalIdentity(ctx, request)
	if err != nil {
		// The provider/account unique index is the concurrency guard. If another
		// request created the same identity first, reuse that completed record.
		credential = findExternalCredential(ctx, request.Provider, request.Subject)
		if credential != nil {
			return loginWithExternalCredential(ctx, credential)
		}
		return nil, err
	}
	user := usermodel.NewUserModel().Find(ctx, map[string]any{
		"id":     userID,
		"status": usermodel.UserStatusEnabled,
	})
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

	account := ""
	if strings.TrimSpace(req.Account) != "" {
		var err error
		account, err = requirePhoneAccount(req.Account)
		if err != nil {
			return ExternalLoginRequest{}, err
		}
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

func bindExternalIdentity(ctx context.Context, req ExternalLoginRequest) (userID uint64, resultErr error) {
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
		user, err := findExternalUserByAccount(tx, req.Account)
		if err != nil {
			return err
		}
		created := false
		if user != nil {
			userID = user.ID
		} else {
			userID = uint64(usermodel.NewUserModel().Insert(tx, map[string]any{
				"account":         req.Account,
				"name":            req.Name,
				"avatar_file_id":  uint64(0),
				"session_version": uint64(1),
				"status":          usermodel.UserStatusEnabled,
				"remark":          "",
				"created_at":      now,
			}))
			if userID == 0 {
				return fmt.Errorf("创建用户失败")
			}
			created = true
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
		if !created {
			return nil
		}
		return initializeRegistrationBenefits(tx, userID, map[string]any{
			"id":      userID,
			"name":    req.Name,
			"account": req.Account,
		}, now)
	})
	return userID, resultErr
}

func findExternalUserByAccount(ctx context.Context, account string) (*usermodel.User, error) {
	account = normalizePhoneAccount(account)
	if account == "" {
		return nil, nil
	}
	users := usermodel.NewUserModel().Select(ctx, map[string]any{
		"account": account,
	}, map[string]any{
		"order": "id asc",
		"limit": 2,
	})
	if len(users) > 1 {
		return nil, fmt.Errorf("该手机号对应多个用户，无法自动绑定")
	}
	if len(users) == 0 || users[0] == nil {
		return nil, nil
	}
	if users[0].Status != usermodel.UserStatusEnabled {
		return nil, fmt.Errorf("该手机号对应的用户已停用")
	}
	return users[0], nil
}

func syncExternalUserAccount(ctx context.Context, userID uint64, account string) error {
	if userID == 0 || account == "" {
		return nil
	}
	account, err := requirePhoneAccount(account)
	if err != nil {
		return err
	}
	userModel := usermodel.NewUserModel()
	user := userModel.Find(ctx, map[string]any{"id": userID})
	if user == nil || user.Account == account {
		return nil
	}
	bound := userModel.Find(ctx, map[string]any{"account": account})
	if bound != nil && bound.ID != userID {
		return fmt.Errorf("该手机号已绑定其他用户")
	}
	return orm.Transaction(ctx, func(tx context.Context) error {
		current := userModel.Find(tx, map[string]any{"id": userID})
		if current == nil {
			return fmt.Errorf("用户不存在")
		}
		userModel.Update(tx, map[string]any{"id": userID}, map[string]any{"account": account})
		passwordCredential := usermodel.NewCredentialModel().Find(tx, map[string]any{
			"user_id":  userID,
			"provider": usermodel.CredentialProviderPassword,
		})
		if passwordCredential != nil {
			conflict := usermodel.NewCredentialModel().Find(tx, map[string]any{
				"provider": usermodel.CredentialProviderPassword,
				"account":  account,
			})
			if conflict != nil && conflict.UserID != userID {
				return fmt.Errorf("该手机号已绑定其他登录凭据")
			}
			usermodel.NewCredentialModel().Update(tx, map[string]any{"id": passwordCredential.ID}, map[string]any{"account": account})
		}
		if err := syncUserPointSnapshots(tx, userID); err != nil {
			return err
		}
		return revokeUserSessions(tx, userID, current.SessionVersion)
	})
}

func truncateRunes(value string, limit int) string {
	chars := []rune(value)
	if limit <= 0 || len(chars) <= limit {
		return value
	}
	return string(chars[:limit])
}
