package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	deverjwt "github.com/shemic/dever/auth/jwt"

	usermodel "github.com/dever-package/user/model"
)

const (
	ActorTypeUser   = "user"
	ActorTypeAPIKey = "api_key"

	TokenScopeUser = "user"
)

type Actor struct {
	ID     uint64
	Type   string
	UserID uint64
	Site   string
	Scope  string
	Scopes []string
}

type actorContextKey struct{}

type AuthRequiredError struct {
	Message string
}

func (e AuthRequiredError) Error() string {
	if message := strings.TrimSpace(e.Message); message != "" {
		return message
	}
	return "请先登录"
}

func NewAuthRequiredError(message string) error {
	return AuthRequiredError{Message: message}
}

func IsAuthRequired(err error) bool {
	var target AuthRequiredError
	return errors.As(err, &target)
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if actor.UserID == 0 {
		return ctx
	}
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	if ctx == nil {
		return Actor{}, false
	}
	if actor, ok := ctx.Value(actorContextKey{}).(Actor); ok && actor.UserID > 0 {
		return actor, true
	}
	return actorFromJWT(ctx)
}

func RequireActor(ctx context.Context) (Actor, error) {
	actor, ok := ActorFromContext(ctx)
	if !ok || actor.UserID == 0 {
		return Actor{}, NewAuthRequiredError("用户信息不正确")
	}
	return actor, nil
}

func CurrentUserID(ctx context.Context) (uint64, error) {
	actor, err := RequireActor(ctx)
	if err != nil {
		return 0, err
	}
	return actor.UserID, nil
}

func CurrentUser(ctx context.Context) (*usermodel.User, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	user := usermodel.NewUserModel().Find(ctx, map[string]any{
		"id":     userID,
		"status": usermodel.UserStatusEnabled,
	})
	if user == nil {
		return nil, NewAuthRequiredError("用户不存在或已停用")
	}
	return user, nil
}

func ActorContext(ctx context.Context, actor Actor) context.Context {
	return WithActor(ctx, actor)
}

func actorFromJWT(ctx context.Context) (Actor, bool) {
	claims := deverjwt.Claims(ctx)
	if len(claims) == 0 {
		return Actor{}, false
	}

	scope := cleanClaim(claims["scope"])
	site := cleanClaim(claims["site"])
	if scope != TokenScopeUser && site != TokenScopeUser {
		return Actor{}, false
	}

	uid, ok := deverjwt.ActiveInt64(ctx)
	if !ok || uid <= 0 {
		return Actor{}, false
	}

	actorType := cleanClaim(claims["actor_type"])
	if actorType == "" {
		actorType = ActorTypeUser
	}
	return Actor{
		ID:     uint64(uid),
		Type:   actorType,
		UserID: uint64(uid),
		Site:   site,
		Scope:  scope,
		Scopes: splitScopes(scope),
	}, true
}

func cleanClaim(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func splitScopes(scope string) []string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(scope, func(r rune) bool {
		return r == ',' || r == ' '
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}
