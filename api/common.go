package api

import (
	"fmt"
	"strings"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	userservice "github.com/dever-package/user/service"
)

func bindBody(c *server.Context) (map[string]any, error) {
	body := map[string]any{}
	if err := c.BindJSON(&body); err != nil {
		return nil, err
	}
	return body, nil
}

func userJSON(c *server.Context, data any, err error) error {
	if err != nil {
		payload := map[string]any{
			"status": 2,
			"data":   map[string]any{},
			"msg":    err.Error(),
		}
		if userservice.IsAuthRequired(err) {
			payload["code"] = 401
		}
		return c.JSONPayload(200, payload)
	}
	return c.JSONPayload(200, map[string]any{
		"status": 1,
		"data":   data,
		"msg":    "",
	})
}

func bodyText(body map[string]any, keys ...string) string {
	for _, key := range keys {
		text := strings.TrimSpace(fmt.Sprint(body[key]))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func bodyUint64(body map[string]any, keys ...string) uint64 {
	for _, key := range keys {
		if value := util.ToUint64(body[key]); value > 0 {
			return value
		}
	}
	return 0
}

func bodyStringSlice(body map[string]any, keys ...string) []string {
	for _, key := range keys {
		switch value := body[key].(type) {
		case []string:
			return value
		case []any:
			result := make([]string, 0, len(value))
			for _, item := range value {
				if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
					result = append(result, text)
				}
			}
			return result
		case string:
			if text := strings.TrimSpace(value); text != "" {
				return strings.Split(text, ",")
			}
		}
	}
	return []string{}
}
