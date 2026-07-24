package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

type PointCheckoutRequest struct {
	PackageID uint64
	RequestID string
}

func (AccountService) CheckoutPointPackage(ctx context.Context, request PointCheckoutRequest) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := normalizeCheckoutRequestID(request.RequestID)
	if err != nil {
		return nil, err
	}
	if request.PackageID == 0 {
		return nil, fmt.Errorf("请选择积分套餐")
	}
	if existing := usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{
		"user_id": userID, "request_id": requestID,
	}); len(existing) > 0 {
		return accountOrderResponse(AccountOrderTypePoint, existing), nil
	}

	now := time.Now()
	orderNo, err := newAccountOrderNo("PT", now)
	if err != nil {
		return nil, err
	}
	userRow := usermodel.NewUserModel().FindMap(ctx, map[string]any{
		"id": userID, "status": usermodel.UserStatusEnabled,
	})
	if len(userRow) == 0 {
		return nil, NewAuthRequiredError("用户不存在或已停用")
	}
	packageRow := usermodel.NewPointPackageModel().FindMap(ctx, map[string]any{
		"id": request.PackageID, "status": identityStatusEnabled,
	})
	if len(packageRow) == 0 {
		return nil, fmt.Errorf("积分套餐不存在或已停用")
	}
	pointAmount := util.ToIntDefault(packageRow["point_amount"], 0)
	bonusAmount := util.ToIntDefault(packageRow["bonus_amount"], 0)
	if err := validatePointChangeAmount(pointAmount, "package_id"); err != nil {
		return nil, err
	}
	if bonusAmount < 0 || bonusAmount > maxPointChangeAmount || pointAmount > maxPointChangeAmount-bonusAmount {
		return nil, fmt.Errorf("积分套餐数量配置不正确")
	}
	pointConfigID := util.ToUint64(packageRow["point_config_id"])
	pointRow := usermodel.NewPointConfigModel().FindMap(ctx, map[string]any{"id": pointConfigID})
	if len(pointRow) == 0 {
		return nil, fmt.Errorf("积分套餐的积分配置不存在")
	}
	payAmountMicros, err := pointAmountToMoneyMicros(pointAmount, util.ToIntDefault(pointRow["exchange_rate"], 0))
	if err != nil {
		return nil, err
	}
	point := pointConfigSnapshot(pointRow)
	record := map[string]any{
		"order_no": orderNo, "request_id": requestID,
		"user_id": userID, "user_name": strings.TrimSpace(util.ToString(userRow["name"])), "user_mobile": strings.TrimSpace(util.ToString(userRow["mobile"])),
		"package_id": request.PackageID, "package_name": strings.TrimSpace(util.ToString(packageRow["name"])),
		"point_config_id": pointConfigID, "point_name": point.name, "point_symbol": point.symbol, "point_symbol_position": point.symbolPosition,
		"point_amount": pointAmount, "bonus_amount": bonusAmount, "total_points": pointAmount + bonusAmount,
		"pay_amount_micros": payAmountMicros, "currency": accountCurrencyCNY,
		"status": usermodel.AccountOrderStatusPendingPayment, "created_at": now,
	}
	orderID, err := insertPointRechargeOrder(ctx, record)
	if err != nil {
		if isUniqueConflictError(err) {
			if existing := usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{
				"user_id": userID, "request_id": requestID,
			}); len(existing) > 0 {
				return accountOrderResponse(AccountOrderTypePoint, existing), nil
			}
		}
		return nil, err
	}
	row := usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{"id": orderID})
	if err := startPointOrderPayment(ctx, row); err != nil {
		return nil, err
	}
	return accountOrderResponse(AccountOrderTypePoint, usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{"id": orderID})), nil
}

func insertPointRechargeOrder(ctx context.Context, record map[string]any) (id uint64, err error) {
	return insertAccountOrder("积分充值订单", func() any {
		return usermodel.NewPointRechargeOrderModel().Insert(ctx, record)
	})
}

func startPointOrderPayment(ctx context.Context, row map[string]any) error {
	status := strings.TrimSpace(util.ToString(row["status"]))
	if !accountOrderCanRetry(status) {
		if status == usermodel.AccountOrderStatusPaying || status == usermodel.AccountOrderStatusCompleted {
			return nil
		}
		return fmt.Errorf("当前积分订单不能发起支付")
	}
	orderID := util.ToUint64(row["id"])
	updated := usermodel.NewPointRechargeOrderModel().Update(ctx, map[string]any{
		"id": orderID, "status": status,
	}, map[string]any{
		"status": usermodel.AccountOrderStatusPaying, "error_message": "",
	}, false)
	if updated == 0 {
		return fmt.Errorf("积分订单状态已变化")
	}
	result, err := createOrderPayment(ctx, PaymentCreateRequest{
		OrderType:    AccountOrderTypePoint,
		OrderNo:      strings.TrimSpace(util.ToString(row["order_no"])),
		Subject:      strings.TrimSpace(util.ToString(row["package_name"])),
		AmountMicros: util.ToInt64(row["pay_amount_micros"]),
		Currency:     strings.TrimSpace(util.ToString(row["currency"])),
	})
	if err != nil {
		usermodel.NewPointRechargeOrderModel().Update(ctx, map[string]any{
			"id": orderID, "status": usermodel.AccountOrderStatusPaying,
		}, map[string]any{
			"status": usermodel.AccountOrderStatusFailed, "error_message": err.Error(),
		}, false)
		return err
	}
	usermodel.NewPointRechargeOrderModel().Update(ctx, map[string]any{
		"id": orderID, "status": usermodel.AccountOrderStatusPaying,
	}, map[string]any{
		"payment_no": result.PaymentNo, "payment_url": result.PaymentURL,
	}, false)
	return nil
}

func CompletePointRechargePayment(ctx context.Context, orderNo string, paymentNo string) (map[string]any, error) {
	orderNo = strings.TrimSpace(orderNo)
	row := usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{"order_no": orderNo})
	if len(row) == 0 {
		return nil, fmt.Errorf("积分充值订单不存在")
	}
	if strings.TrimSpace(util.ToString(row["status"])) == usermodel.AccountOrderStatusCompleted {
		return accountOrderResponse(AccountOrderTypePoint, row), nil
	}
	existingPaymentNo := strings.TrimSpace(util.ToString(row["payment_no"]))
	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		return nil, fmt.Errorf("支付单号不能为空")
	}
	if existingPaymentNo != "" && existingPaymentNo != paymentNo {
		return nil, fmt.Errorf("支付单号不匹配")
	}
	now := time.Now()
	updated := usermodel.NewPointRechargeOrderModel().Update(ctx, map[string]any{
		"id":     util.ToUint64(row["id"]),
		"status": []any{usermodel.AccountOrderStatusPendingPayment, usermodel.AccountOrderStatusPaying, usermodel.AccountOrderStatusFailed},
	}, map[string]any{
		"status": usermodel.AccountOrderStatusPaid, "payment_no": paymentNo, "error_message": "", "paid_at": now,
	}, false)
	if updated == 0 {
		latest := usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{"id": util.ToUint64(row["id"])})
		latestStatus := strings.TrimSpace(util.ToString(latest["status"]))
		if latestStatus == usermodel.AccountOrderStatusCompleted {
			return accountOrderResponse(AccountOrderTypePoint, latest), nil
		}
		if latestStatus != usermodel.AccountOrderStatusPaid {
			return nil, fmt.Errorf("积分订单状态已变化")
		}
	}
	if err := fulfillPointRechargeOrder(ctx, orderNo); err != nil {
		latest := usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{"order_no": orderNo})
		if strings.TrimSpace(util.ToString(latest["status"])) == usermodel.AccountOrderStatusCompleted {
			return accountOrderResponse(AccountOrderTypePoint, latest), nil
		}
		return nil, err
	}
	return accountOrderResponse(AccountOrderTypePoint, usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{"order_no": orderNo})), nil
}

func fulfillPointRechargeOrder(ctx context.Context, orderNo string) error {
	return orm.Transaction(ctx, func(txCtx context.Context) error {
		model := usermodel.NewPointRechargeOrderModel()
		row := model.FindMap(txCtx, map[string]any{"order_no": strings.TrimSpace(orderNo)})
		if len(row) == 0 {
			return fmt.Errorf("积分充值订单不存在")
		}
		status := strings.TrimSpace(util.ToString(row["status"]))
		if status == usermodel.AccountOrderStatusCompleted {
			return nil
		}
		if status != usermodel.AccountOrderStatusPaid {
			return fmt.Errorf("积分订单尚未支付")
		}
		orderID := util.ToUint64(row["id"])
		if model.Update(txCtx, map[string]any{
			"id": orderID, "status": usermodel.AccountOrderStatusPaid,
		}, map[string]any{
			"status": usermodel.AccountOrderStatusFulfilling,
		}, false) == 0 {
			latest := model.FindMap(txCtx, map[string]any{"id": orderID})
			if strings.TrimSpace(util.ToString(latest["status"])) == usermodel.AccountOrderStatusCompleted {
				return nil
			}
			return fmt.Errorf("积分订单正在处理中")
		}
		totalPoints := util.ToIntDefault(row["total_points"], 0)
		if _, err := adjustUserPointsByOperation(txCtx, "point-order:"+orderNo+":credit", pointAdjustRequest{
			userID: util.ToUint64(row["user_id"]), pointConfigID: util.ToUint64(row["point_config_id"]),
			changeType: pointChangeIncrease, source: pointSourcePurchase, amount: totalPoints,
			remark: "积分充值订单 " + orderNo, createdAt: time.Now(),
		}); err != nil {
			return err
		}
		now := time.Now()
		if model.Update(txCtx, map[string]any{
			"id": orderID, "status": usermodel.AccountOrderStatusFulfilling,
		}, map[string]any{
			"status": usermodel.AccountOrderStatusCompleted, "fulfilled_at": now, "error_message": "",
		}, false) == 0 {
			return fmt.Errorf("积分订单完成状态保存失败")
		}
		return nil
	})
}
