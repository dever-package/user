package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

const (
	AccountOrderTypeIdentity = "identity"
	AccountOrderTypePoint    = "point"

	identityOrderActionSubscribe = "subscribe"
	identityOrderActionRenew     = "renew"
	identityOrderActionSwitch    = "switch"

	pointSourcePurchase = "purchase"
	accountCurrencyCNY  = "CNY"
	accountMoneyMicros  = int64(1_000_000)
)

type PaymentCreateRequest struct {
	OrderType    string
	OrderNo      string
	Subject      string
	AmountMicros int64
	Currency     string
}

type PaymentCreateResult struct {
	PaymentNo  string
	PaymentURL string
}

// Implementations must treat OrderNo and PaymentNo as idempotency keys.
type PaymentGateway interface {
	CreatePayment(context.Context, PaymentCreateRequest) (PaymentCreateResult, error)
	CancelPayment(context.Context, string) error
}

var accountPaymentGateway struct {
	sync.RWMutex
	value PaymentGateway
}

func RegisterPaymentGateway(gateway PaymentGateway) {
	accountPaymentGateway.Lock()
	accountPaymentGateway.value = gateway
	accountPaymentGateway.Unlock()
}

func registeredPaymentGateway() PaymentGateway {
	accountPaymentGateway.RLock()
	defer accountPaymentGateway.RUnlock()
	return accountPaymentGateway.value
}

func createOrderPayment(ctx context.Context, request PaymentCreateRequest) (PaymentCreateResult, error) {
	gateway := registeredPaymentGateway()
	if gateway == nil {
		return PaymentCreateResult{}, fmt.Errorf("支付服务未配置")
	}
	result, err := gateway.CreatePayment(ctx, request)
	if err != nil {
		return PaymentCreateResult{}, err
	}
	result.PaymentNo = strings.TrimSpace(result.PaymentNo)
	result.PaymentURL = strings.TrimSpace(result.PaymentURL)
	if result.PaymentNo == "" {
		return PaymentCreateResult{}, fmt.Errorf("支付服务未返回支付单号")
	}
	return result, nil
}

func cancelOrderPayment(ctx context.Context, paymentNo string) error {
	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		return nil
	}
	gateway := registeredPaymentGateway()
	if gateway == nil {
		return fmt.Errorf("当前支付服务不支持取消支付")
	}
	return gateway.CancelPayment(ctx, paymentNo)
}

func normalizeCheckoutRequestID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("请求标识不能为空")
	}
	if len(value) > 64 {
		return "", fmt.Errorf("请求标识不能超过 64 个字符")
	}
	return value, nil
}

func newAccountOrderNo(prefix string, now time.Time) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("生成订单号失败: %w", err)
	}
	return strings.ToUpper(prefix) + now.UTC().Format("20060102150405") + hex.EncodeToString(random), nil
}

func insertAccountOrder(name string, insert func() any) (id uint64, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok {
				err = recoveredErr
				return
			}
			err = fmt.Errorf("%v", recovered)
		}
	}()
	id = util.ToUint64(insert())
	if id == 0 {
		return 0, fmt.Errorf("%s保存失败", name)
	}
	return id, nil
}

func pointAmountToMoneyMicros(points int, exchangeRate int) (int64, error) {
	if points <= 0 {
		return 0, fmt.Errorf("支付积分必须大于 0")
	}
	if exchangeRate <= 0 {
		return 0, fmt.Errorf("当前积分未配置支付货币换算")
	}
	numerator := int64(points) * accountMoneyMicros
	return (numerator + int64(exchangeRate) - 1) / int64(exchangeRate), nil
}

func accountOrderCanCancel(status string) bool {
	switch strings.TrimSpace(status) {
	case usermodel.AccountOrderStatusPendingPayment, usermodel.AccountOrderStatusPaying, usermodel.AccountOrderStatusFailed:
		return true
	default:
		return false
	}
}

func accountOrderCanRetry(status string) bool {
	switch strings.TrimSpace(status) {
	case usermodel.AccountOrderStatusPendingPayment, usermodel.AccountOrderStatusFailed, usermodel.AccountOrderStatusPaid:
		return true
	default:
		return false
	}
}

func normalizeAccountOrderType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AccountOrderTypeIdentity:
		return AccountOrderTypeIdentity
	case AccountOrderTypePoint:
		return AccountOrderTypePoint
	default:
		return ""
	}
}

func accountOrderTime(value any) string {
	parsed := normalizeUserIdentityTime(value)
	if parsed.IsZero() {
		return ""
	}
	return parsed.Format(time.RFC3339)
}

func accountOrderResponse(orderType string, row map[string]any) map[string]any {
	status := strings.TrimSpace(util.ToString(row["status"]))
	paymentURL := ""
	if status == usermodel.AccountOrderStatusPaying {
		paymentURL = strings.TrimSpace(util.ToString(row["payment_url"]))
	}
	result := map[string]any{
		"id":                util.ToUint64(row["id"]),
		"type":              orderType,
		"order_no":          strings.TrimSpace(util.ToString(row["order_no"])),
		"point_config_id":   util.ToUint64(row["point_config_id"]),
		"point_name":        strings.TrimSpace(util.ToString(row["point_name"])),
		"status":            status,
		"payment_url":       paymentURL,
		"pay_amount_micros": util.ToInt64(row["pay_amount_micros"]),
		"currency":          strings.TrimSpace(util.ToString(row["currency"])),
		"error":             strings.TrimSpace(util.ToString(row["error_message"])),
		"created_at":        accountOrderTime(row["created_at"]),
		"paid_at":           accountOrderTime(row["paid_at"]),
		"fulfilled_at":      accountOrderTime(row["fulfilled_at"]),
	}
	result["can_cancel"] = accountOrderCanCancel(status)
	result["can_retry"] = accountOrderCanRetry(status)
	if orderType == AccountOrderTypeIdentity {
		result["title"] = strings.TrimSpace(util.ToString(row["identity_name"])) + " · " + strings.TrimSpace(util.ToString(row["level_name"]))
		result["action"] = strings.TrimSpace(util.ToString(row["action"]))
		result["total_points"] = util.ToIntDefault(row["total_points"], 0)
		result["recharge_points"] = util.ToIntDefault(row["recharge_points"], 0)
		result["target_expired_at"] = accountOrderTime(row["target_expired_at"])
	} else {
		result["title"] = strings.TrimSpace(util.ToString(row["package_name"]))
		result["total_points"] = util.ToIntDefault(row["total_points"], 0)
		result["bonus_points"] = util.ToIntDefault(row["bonus_amount"], 0)
	}
	return result
}
