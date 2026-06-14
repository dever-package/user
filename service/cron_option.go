package service

import "github.com/shemic/dever/server"

type CronOptionService struct{}

func (CronOptionService) ProviderLoadCronProviders(_ *server.Context, _ []any) any {
	return []map[string]any{
		{
			"id":    "user.CronService.IssueIdentityBenefits",
			"value": "身份周期权益发放",
		},
		{
			"id":    "front.cron.CronService.EchoHello",
			"value": "测试 Echo Hello",
		},
	}
}
