package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shemic/dever/orm"

	usermodel "github.com/dever-package/user/model"
)

type AdminUserService struct{}

type AdminUserSaveRequest struct {
	ID       uint64 `json:"id"`
	Account  string `json:"account"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Status   int16  `json:"status"`
	Remark   string `json:"remark"`
}

func (AdminUserService) Save(ctx context.Context, req AdminUserSaveRequest) (result map[string]any, resultErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok {
				resultErr = recoveredErr
			} else {
				resultErr = fmt.Errorf("%v", recovered)
			}
			result = nil
		}
	}()

	account, err := requirePhoneAccount(req.Account)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("姓名不能为空")
	}
	if len([]rune(name)) > 64 {
		return nil, fmt.Errorf("姓名不能超过 64 个字符")
	}
	password := strings.TrimSpace(req.Password)
	if req.ID == 0 && password == "" {
		return nil, fmt.Errorf("新增用户必须设置密码")
	}
	if password != "" && len([]rune(password)) < 6 {
		return nil, fmt.Errorf("密码不能少于 6 位")
	}
	passwordHash := ""
	if password != "" {
		passwordHash, err = hashPassword(password)
		if err != nil {
			return nil, err
		}
	}

	created := req.ID == 0
	userID := req.ID
	err = orm.Transaction(ctx, func(tx context.Context) error {
		if err := ensureUserAccountAvailable(tx, account, userID); err != nil {
			return err
		}

		userModel := usermodel.NewUserModel()
		if created {
			status, err := normalizeAdminUserStatus(req.Status, usermodel.UserStatusEnabled)
			if err != nil {
				return err
			}
			now := time.Now()
			userID = uint64(userModel.Insert(tx, map[string]any{
				"account":         account,
				"name":            name,
				"avatar_file_id":  uint64(0),
				"session_version": uint64(1),
				"status":          status,
				"remark":          strings.TrimSpace(req.Remark),
				"created_at":      now,
			}))
			if userID == 0 {
				return fmt.Errorf("新增用户失败")
			}
			if err := saveAdminPasswordCredential(tx, userID, account, passwordHash); err != nil {
				return err
			}
			if err := syncUserPointSnapshots(tx, userID); err != nil {
				return err
			}
			return initializeRegistrationBenefits(tx, userID, map[string]any{
				"id":      userID,
				"name":    name,
				"account": account,
			}, now)
		}

		current := userModel.Find(tx, map[string]any{"id": userID})
		if current == nil {
			return fmt.Errorf("用户不存在")
		}
		status, err := normalizeAdminUserStatus(req.Status, current.Status)
		if err != nil {
			return err
		}
		securityChanged := current.Account != account || current.Status != status || passwordHash != ""
		userModel.Update(tx, map[string]any{"id": userID}, map[string]any{
			"account": account,
			"name":    name,
			"status":  status,
			"remark":  strings.TrimSpace(req.Remark),
		})
		if err := saveAdminPasswordCredential(tx, userID, account, passwordHash); err != nil {
			return err
		}
		if securityChanged {
			if err := revokeUserSessions(tx, userID, current.SessionVersion); err != nil {
				return err
			}
		}
		return syncUserPointSnapshots(tx, userID)
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"id":      userID,
		"created": created,
	}, nil
}

func normalizeAdminUserStatus(status int16, fallback int16) (int16, error) {
	if status == 0 {
		status = fallback
	}
	if status != usermodel.UserStatusEnabled && status != usermodel.UserStatusDisabled {
		return 0, fmt.Errorf("用户状态无效")
	}
	return status, nil
}

func saveAdminPasswordCredential(ctx context.Context, userID uint64, account string, passwordHash string) error {
	credentialModel := usermodel.NewCredentialModel()
	credential := credentialModel.Find(ctx, map[string]any{
		"user_id":  userID,
		"provider": usermodel.CredentialProviderPassword,
	})
	if credential == nil {
		if passwordHash == "" {
			return nil
		}
		credentialID := credentialModel.Insert(ctx, map[string]any{
			"user_id":       userID,
			"provider":      usermodel.CredentialProviderPassword,
			"account":       account,
			"password_hash": passwordHash,
			"status":        usermodel.CredentialStatusEnabled,
			"created_at":    time.Now(),
		})
		if credentialID == 0 {
			return fmt.Errorf("登录密码保存失败")
		}
		return nil
	}

	updates := map[string]any{"account": account}
	if passwordHash != "" {
		updates["password_hash"] = passwordHash
		updates["status"] = usermodel.CredentialStatusEnabled
	}
	credentialModel.Update(ctx, map[string]any{"id": credential.ID}, updates)
	return nil
}
