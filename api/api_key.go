package api

import (
	"time"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	userservice "my/package/user/service"
)

type ApiKey struct{}

func (ApiKey) GetList(c *server.Context) error {
	data, err := (userservice.APIKeyService{}).List(c.Context())
	return userJSON(c, data, err)
}

func (ApiKey) PostCreate(c *server.Context) error {
	body, err := bindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.APIKeyService{}).Create(c.Context(), userservice.CreateAPIKeyRequest{
		Name:      bodyText(body, "name"),
		Scopes:    bodyStringSlice(body, "scopes", "scope"),
		ExpiresAt: parseTime(body["expires_at"], body["expiresAt"]),
	})
	return userJSON(c, data, err)
}

func (ApiKey) PostDisable(c *server.Context) error {
	body, err := bindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.APIKeyService{}).Disable(c.Context(), bodyUint64(body, "id", "api_key_id", "apiKeyId"))
	return userJSON(c, data, err)
}

func parseTime(values ...any) time.Time {
	for _, value := range values {
		text := util.ToStringTrimmed(value)
		if text == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.ParseInLocation(layout, text, time.Local); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}
