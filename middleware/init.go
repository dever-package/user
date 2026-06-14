package middleware

import (
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
			return c.Error(err)
		}
		c.SetContext(userservice.WithActor(c.Context(), actor))
		return nil
	}
}

func requestAPIKey(c *server.Context) string {
	return authctx.RequestAPIKey(c)
}
