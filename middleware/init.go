package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	coremiddleware "github.com/shemic/dever/middleware"
	"github.com/shemic/dever/server"

	"my/package/user/authctx"
	userservice "my/package/user/service"
)

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		coremiddleware.UseGlobalFunc(apiKeyActor())
	})
}

func apiKeyActor() coremiddleware.ContextFunc {
	return func(ctx any) error {
		c, ok := ctx.(*server.Context)
		if !ok || c == nil {
			return nil
		}
		secret := requestAPIKey(c)
		if secret == "" {
			return nil
		}
		actor, err := (userservice.APIKeyService{}).Authenticate(c.Context(), secret)
		if err != nil {
			return abortUnauthorized(c, err.Error())
		}
		if !apiKeyAllowsPath(actor, c.Path()) {
			return abortUnauthorized(c, "API Key 无权访问当前接口")
		}
		c.SetContext(userservice.WithActor(c.Context(), actor))
		return nil
	}
}

func requestAPIKey(c *server.Context) string {
	return authctx.RequestAPIKey(c)
}

func apiKeyAllowsPath(actor userservice.Actor, requestPath string) bool {
	if actor.Type != userservice.ActorTypeAPIKey {
		return true
	}
	parts := requestPathScopeParts(requestPath)
	if len(parts) == 0 {
		return true
	}
	for _, scope := range actor.Scopes {
		if scopeAllowsPath(scope, parts) {
			return true
		}
	}
	return false
}

func requestPathScopeParts(requestPath string) []string {
	requestPath = strings.Trim(strings.TrimSpace(requestPath), "/")
	if requestPath == "" {
		return nil
	}
	parts := strings.Split(requestPath, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func scopeAllowsPath(scope string, parts []string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return false
	}
	if scope == "*" {
		return true
	}
	scopeParts := strings.Split(scope, ":")
	for index, part := range scopeParts {
		part = strings.TrimSpace(part)
		if part == "" {
			return false
		}
		if part == "*" {
			return true
		}
		if index >= len(parts) || part != parts[index] {
			return false
		}
	}
	return len(scopeParts) <= len(parts)
}

func abortUnauthorized(c *server.Context, msg string) error {
	if c != nil {
		_ = c.Error(msg, http.StatusUnauthorized)
		panic(server.Abort{Err: fmt.Errorf("%s", msg)})
	}
	return fmt.Errorf("%s", msg)
}
