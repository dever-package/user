package model

const (
	AccountOrderStatusPendingPayment = "pending_payment"
	AccountOrderStatusPaying         = "paying"
	AccountOrderStatusPaid           = "paid"
	AccountOrderStatusFulfilling     = "fulfilling"
	AccountOrderStatusCompleted      = "completed"
	AccountOrderStatusFailed         = "failed"
	AccountOrderStatusCanceled       = "canceled"
)

var accountOrderStatusOptions = []map[string]any{
	{"id": AccountOrderStatusPendingPayment, "value": "待支付", "label": "待支付", "color": "#ea580c"},
	{"id": AccountOrderStatusPaying, "value": "支付中", "label": "支付中", "color": "#2563eb"},
	{"id": AccountOrderStatusPaid, "value": "已支付", "label": "已支付", "color": "#2563eb"},
	{"id": AccountOrderStatusFulfilling, "value": "履约中", "label": "履约中", "color": "#7c3aed"},
	{"id": AccountOrderStatusCompleted, "value": "已完成", "label": "已完成", "color": "#0f766e"},
	{"id": AccountOrderStatusFailed, "value": "失败", "label": "失败", "color": "#dc2626"},
	{"id": AccountOrderStatusCanceled, "value": "已取消", "label": "已取消", "color": "#737373"},
}
