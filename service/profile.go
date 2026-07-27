package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shemic/dever/orm"

	uploadrepo "github.com/dever-package/front/service/upload/repository"
	usermodel "github.com/dever-package/user/model"
)

const (
	profileAvatarRuleID    = uint64(1)
	profileAvatarBizPrefix = "user_avatar_"
)

type ProfileService struct{}

type UpdateProfileRequest struct {
	Name            string
	AvatarFileID    uint64
	AvatarFileIDSet bool
}

type ChangePasswordRequest struct {
	CurrentPassword string
	NewPassword     string
}

func ProfileAvatarBizKey(userID uint64) string {
	return fmt.Sprintf("%s%d", profileAvatarBizPrefix, userID)
}

func ProfileAvatarBizPrefix() string {
	return profileAvatarBizPrefix
}

func (ProfileService) Update(ctx context.Context, req UpdateProfileRequest) (map[string]any, error) {
	user, err := CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("昵称不能为空")
	}
	if len([]rune(name)) > 64 {
		return nil, fmt.Errorf("昵称不能超过 64 个字符")
	}

	updates := map[string]any{"name": name}
	if req.AvatarFileIDSet {
		if req.AvatarFileID > 0 {
			if err := validateProfileAvatar(ctx, user.ID, req.AvatarFileID); err != nil {
				return nil, err
			}
		}
		updates["avatar_file_id"] = req.AvatarFileID
	}
	usermodel.NewUserModel().Update(ctx, map[string]any{
		"id":     user.ID,
		"status": usermodel.UserStatusEnabled,
	}, updates)

	updated := usermodel.NewUserModel().Find(ctx, map[string]any{
		"id":     user.ID,
		"status": usermodel.UserStatusEnabled,
	})
	if updated == nil {
		return nil, fmt.Errorf("用户不存在或已停用")
	}
	return map[string]any{"user": userPayload(ctx, *updated, "")}, nil
}

func (ProfileService) ChangePassword(ctx context.Context, req ChangePasswordRequest) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if currentPassword == "" {
		return nil, fmt.Errorf("请输入当前密码")
	}
	if len([]rune(newPassword)) < 6 {
		return nil, fmt.Errorf("新密码不能少于 6 位")
	}

	credentialModel := usermodel.NewCredentialModel()
	credential := credentialModel.Find(ctx, map[string]any{
		"user_id":  userID,
		"provider": usermodel.CredentialProviderPassword,
		"status":   usermodel.CredentialStatusEnabled,
	})
	if credential == nil || !verifyPassword(currentPassword, credential.PasswordHash) {
		return nil, fmt.Errorf("当前密码不正确")
	}
	if verifyPassword(newPassword, credential.PasswordHash) {
		return nil, fmt.Errorf("新密码不能与当前密码相同")
	}
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	if err := orm.Transaction(ctx, func(tx context.Context) error {
		currentCredential := credentialModel.Find(tx, map[string]any{
			"id":       credential.ID,
			"user_id":  userID,
			"provider": usermodel.CredentialProviderPassword,
			"status":   usermodel.CredentialStatusEnabled,
		})
		if currentCredential == nil || !verifyPassword(currentPassword, currentCredential.PasswordHash) {
			return fmt.Errorf("当前密码不正确")
		}
		if credentialModel.Update(tx, map[string]any{
			"id":            currentCredential.ID,
			"password_hash": currentCredential.PasswordHash,
		}, map[string]any{"password_hash": passwordHash}) == 0 {
			return fmt.Errorf("密码修改失败，请重试")
		}

		user := usermodel.NewUserModel().Find(tx, map[string]any{
			"id":     userID,
			"status": usermodel.UserStatusEnabled,
		})
		if user == nil {
			return NewAuthRequiredError("用户不存在或已停用")
		}
		return revokeUserSessions(tx, userID, user.SessionVersion)
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func revokeUserSessions(ctx context.Context, userID uint64, sessionVersion uint64) error {
	if usermodel.NewUserModel().Update(ctx, map[string]any{
		"id":              userID,
		"session_version": sessionVersion,
	}, map[string]any{
		"session_version": normalizeSessionVersion(sessionVersion) + 1,
	}) == 0 {
		return fmt.Errorf("用户登录状态已变化，请重试")
	}
	usermodel.NewTokenModel().Update(ctx, map[string]any{
		"user_id": userID,
		"type":    usermodel.TokenTypeRefresh,
		"status":  usermodel.TokenStatusEnabled,
	}, map[string]any{
		"status":  usermodel.TokenStatusRevoked,
		"used_at": time.Now(),
	})
	return nil
}

func validateProfileAvatar(ctx context.Context, userID uint64, fileID uint64) error {
	file, err := uploadrepo.FindUploadFile(ctx, fileID)
	if err != nil {
		return fmt.Errorf("头像文件不存在")
	}
	if file.RuleID != profileAvatarRuleID || file.Kind != "image" || file.BizKey != ProfileAvatarBizKey(userID) {
		return fmt.Errorf("头像文件无效")
	}
	return nil
}

func userAvatarPayload(ctx context.Context, fileID uint64) (uint64, string) {
	if fileID == 0 {
		return 0, ""
	}
	file, err := uploadrepo.FindUploadFile(ctx, fileID)
	if err != nil || file.Kind != "image" {
		return 0, ""
	}
	payload := uploadrepo.BuildUploadFilePayload(file)
	for _, key := range []string{"url", "thumbnail", "open_url"} {
		if value := strings.TrimSpace(fmt.Sprint(payload[key])); value != "" && value != "<nil>" {
			return file.ID, value
		}
	}
	return 0, ""
}
