package authctx

import (
	"strings"

	"github.com/shemic/dever/server"
)

const APIKeyHeader = "X-API-Key"

func RequestAPIKey(c *server.Context) string {
	if c == nil {
		return ""
	}
	if value := strings.TrimSpace(c.Header(APIKeyHeader)); value != "" {
		return value
	}
	auth := strings.TrimSpace(c.Header("Authorization"))
	if auth == "" {
		return ""
	}
	lower := strings.ToLower(auth)
	if strings.HasPrefix(lower, "bearer ") {
		auth = strings.TrimSpace(auth[len("bearer "):])
	}
	if strings.HasPrefix(auth, "uapi_") {
		return auth
	}
	return ""
}

func HasAPIKey(c *server.Context) bool {
	return RequestAPIKey(c) != ""
}
