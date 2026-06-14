package model

var userStatusOptions = []map[string]any{
	{"id": 1, "value": "正常", "label": "正常", "color": "#0f766e"},
	{"id": 2, "value": "禁用", "label": "禁用", "color": "#737373"},
}

var credentialProviderOptions = []map[string]any{
	{"id": "password", "value": "密码登录", "label": "密码登录", "color": "#2563eb"},
}

var credentialStatusOptions = []map[string]any{
	{"id": 1, "value": "正常", "label": "正常", "color": "#0f766e"},
	{"id": 2, "value": "禁用", "label": "禁用", "color": "#737373"},
}

var tokenStatusOptions = []map[string]any{
	{"id": 1, "value": "有效", "label": "有效", "color": "#0f766e"},
	{"id": 2, "value": "撤销", "label": "撤销", "color": "#737373"},
}

var apiKeyStatusOptions = []map[string]any{
	{"id": 1, "value": "启用", "label": "启用", "color": "#0f766e"},
	{"id": 2, "value": "停用", "label": "停用", "color": "#737373"},
}

var pointChangeTypeOptions = []map[string]any{
	{"id": "increase", "value": "增加积分", "label": "增加积分", "color": "#0f766e"},
	{"id": "consume", "value": "消耗积分", "label": "消耗积分", "color": "#dc2626"},
}

var pointSourceOptions = []map[string]any{
	{"id": "admin", "value": "后台调整", "label": "后台调整", "color": "#2563eb"},
	{"id": "system", "value": "系统变动", "label": "系统变动", "color": "#737373"},
	{"id": "cron", "value": "计划任务", "label": "计划任务", "color": "#7c3aed"},
}

var benefitTypeOptions = []map[string]any{
	{"id": "reward_point", "value": "奖励积分", "label": "奖励积分", "color": "#0f766e"},
}

var benefitClearPreviousOptions = []map[string]any{
	{"id": 1, "value": "清空", "label": "清空", "color": "#dc2626"},
	{"id": 2, "value": "不清空", "label": "不清空", "color": "#0f766e"},
}

var benefitGrantStatusOptions = []map[string]any{
	{"id": 1, "value": "生效", "label": "生效", "color": "#0f766e"},
	{"id": 2, "value": "已清空", "label": "已清空", "color": "#737373"},
}

var identityStatusOptions = []map[string]any{
	{"id": 1, "value": "启用", "label": "启用", "color": "#0f766e"},
	{"id": 2, "value": "停用", "label": "停用", "color": "#737373"},
}

var levelDurationTypeOptions = []map[string]any{
	{"id": 1, "value": "重计时长", "label": "重计时长", "color": "#2563eb"},
	{"id": 2, "value": "续费时长", "label": "续费时长", "color": "#0f766e"},
}

var levelUpgradeMethodOptions = []map[string]any{
	{"id": 1, "value": "支付", "label": "支付", "color": "#2563eb"},
	{"id": 2, "value": "注册", "label": "注册", "color": "#7c3aed"},
	{"id": 3, "value": "手动赠送", "label": "手动赠送", "color": "#0f766e"},
}

var levelPayTypeOptions = []map[string]any{
	{"id": 1, "value": "差额支付", "label": "差额支付", "color": "#ea580c"},
	{"id": 2, "value": "全额支付", "label": "全额支付", "color": "#2563eb"},
}

var pointSymbolPositionOptions = []map[string]any{
	{"id": 1, "value": "前", "label": "前", "color": "#2563eb"},
	{"id": 2, "value": "后", "label": "后", "color": "#0f766e"},
}
