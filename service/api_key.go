package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	usermodel "github.com/dever-package/user/model"
)

type APIKeyService struct{}

type CreateAPIKeyRequest struct {
	Name      string
	Scopes    []string
	ExpiresAt time.Time
}

func (APIKeyService) List(ctx context.Context) (map[string]any, error) {
	actor, err := RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	rows := usermodel.NewAPIKeyModel().Select(ctx, map[string]any{"user_id": actor.UserID})
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, apiKeyPayload(*row, false))
	}
	return map[string]any{"list": items}, nil
}

func (APIKeyService) Create(ctx context.Context, req CreateAPIKeyRequest) (map[string]any, error) {
	actor, err := RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "API Key"
	}
	secret, err := newAPIKey()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	keyID := uint64(usermodel.NewAPIKeyModel().Insert(ctx, map[string]any{
		"user_id":      actor.UserID,
		"name":         name,
		"prefix":       visiblePrefix(secret),
		"key_hash":     hashSecret(secret),
		"scopes":       encodeScopes(req.Scopes),
		"status":       usermodel.APIKeyStatusEnabled,
		"expires_at":   req.ExpiresAt,
		"last_used_at": time.Time{},
		"created_at":   now,
	}))
	if keyID == 0 {
		return nil, fmt.Errorf("创建API Key失败")
	}
	row := usermodel.NewAPIKeyModel().Find(ctx, map[string]any{"id": keyID})
	if row == nil {
		return nil, fmt.Errorf("读取API Key失败")
	}
	payload := apiKeyPayload(*row, true)
	payload["key"] = secret
	return payload, nil
}

func (APIKeyService) Disable(ctx context.Context, id uint64) (map[string]any, error) {
	actor, err := RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, fmt.Errorf("API Key不能为空")
	}
	affected := usermodel.NewAPIKeyModel().Update(ctx, map[string]any{
		"id":      id,
		"user_id": actor.UserID,
	}, map[string]any{
		"status": usermodel.APIKeyStatusDisabled,
	})
	if affected == 0 {
		return nil, fmt.Errorf("API Key不存在")
	}
	return map[string]any{"ok": true}, nil
}

func (APIKeyService) Authenticate(ctx context.Context, secret string) (Actor, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return Actor{}, NewAuthRequiredError("缺少API Key")
	}
	row := usermodel.NewAPIKeyModel().Find(ctx, map[string]any{
		"key_hash": hashSecret(secret),
		"status":   usermodel.APIKeyStatusEnabled,
	})
	if row == nil {
		return Actor{}, NewAuthRequiredError("API Key无效")
	}
	if !row.ExpiresAt.IsZero() && !row.ExpiresAt.After(time.Now()) {
		return Actor{}, NewAuthRequiredError("API Key已过期")
	}
	user := usermodel.NewUserModel().Find(ctx, map[string]any{
		"id":     row.UserID,
		"status": usermodel.UserStatusEnabled,
	})
	if user == nil {
		return Actor{}, NewAuthRequiredError("用户不存在或已停用")
	}
	usermodel.NewAPIKeyModel().Update(ctx, map[string]any{"id": row.ID}, map[string]any{
		"last_used_at": time.Now(),
	})
	return Actor{
		ID:     row.ID,
		Type:   ActorTypeAPIKey,
		UserID: row.UserID,
		Scope:  TokenScopeUser,
		Scopes: decodeScopes(row.Scopes),
	}, nil
}

func apiKeyPayload(row usermodel.APIKey, includeSecret bool) map[string]any {
	payload := map[string]any{
		"id":           row.ID,
		"name":         row.Name,
		"prefix":       row.Prefix,
		"scopes":       decodeScopes(row.Scopes),
		"status":       row.Status,
		"expires_at":   row.ExpiresAt,
		"last_used_at": row.LastUsedAt,
		"created_at":   row.CreatedAt,
	}
	if includeSecret {
		payload["key"] = ""
	}
	return payload
}

func decodeScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{TokenScopeUser}
	}
	var scopes []string
	if err := json.Unmarshal([]byte(raw), &scopes); err != nil {
		return []string{TokenScopeUser}
	}
	cleaned := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if item := strings.TrimSpace(scope); item != "" {
			cleaned = append(cleaned, item)
		}
	}
	if len(cleaned) == 0 {
		return []string{TokenScopeUser}
	}
	return cleaned
}
