package service

import (
	"context"
	"time"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"
)

type CronService struct{}

func (CronService) ProviderIssueIdentityBenefits(c *server.Context, params []any) any {
	payload := cronPayload(params)
	now := normalizeUserIdentityTime(payload["now"])
	if now.IsZero() {
		now = normalizeUserIdentityTime(payload["run_at"])
	}
	if now.IsZero() {
		now = time.Now()
	}

	result, err := (BenefitService{}).IssueDueIdentityBenefits(cronContext(c, params), now)
	if err != nil {
		panic(err)
	}
	return result
}

func cronContext(c *server.Context, params []any) context.Context {
	if c != nil {
		return c.Context()
	}
	for _, item := range params {
		if ctx, ok := item.(context.Context); ok && ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func cronPayload(params []any) map[string]any {
	for _, item := range params {
		if row, ok := item.(map[string]any); ok && row != nil {
			return util.CloneMap(row)
		}
	}
	return map[string]any{}
}
