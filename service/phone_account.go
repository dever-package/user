package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	usermodel "github.com/dever-package/user/model"
)

var phoneAccountPattern = regexp.MustCompile(`^1\d{10}$`)

func normalizePhoneAccount(value string) string {
	account := strings.NewReplacer(
		" ", "",
		"-", "",
		"(", "",
		")", "",
	).Replace(strings.TrimSpace(value))
	switch {
	case strings.HasPrefix(account, "+86") && len(account) == 14:
		return account[3:]
	case strings.HasPrefix(account, "0086") && len(account) == 15:
		return account[4:]
	case strings.HasPrefix(account, "86") && len(account) == 13:
		return account[2:]
	default:
		return account
	}
}

func requirePhoneAccount(value string) (string, error) {
	account := normalizePhoneAccount(value)
	if !phoneAccountPattern.MatchString(account) {
		return "", fmt.Errorf("手机号格式不正确")
	}
	return account, nil
}

func ensureUserAccountAvailable(ctx context.Context, account string, userID uint64) error {
	if user := usermodel.NewUserModel().Find(ctx, map[string]any{"account": account}); user != nil && user.ID != userID {
		return fmt.Errorf("该手机号已存在")
	}
	credential := usermodel.NewCredentialModel().Find(ctx, map[string]any{
		"provider": usermodel.CredentialProviderPassword,
		"account":  account,
	})
	if credential != nil && credential.UserID != userID {
		return fmt.Errorf("该手机号已绑定其他登录凭据")
	}
	return nil
}
