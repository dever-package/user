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

type SubscriptionCheckoutRequest struct {
	LevelID   uint64
	RequestID string
}

func (AccountService) CheckoutSubscription(ctx context.Context, request SubscriptionCheckoutRequest) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := normalizeCheckoutRequestID(request.RequestID)
	if err != nil {
		return nil, err
	}
	if request.LevelID == 0 {
		return nil, fmt.Errorf("请选择订阅方案")
	}
	if existing := usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{
		"user_id": userID, "request_id": requestID,
	}); len(existing) > 0 {
		return accountOrderResponse(AccountOrderTypeIdentity, existing), nil
	}

	now := time.Now()
	orderNo, err := newAccountOrderNo("ID", now)
	if err != nil {
		return nil, err
	}
	var orderID uint64
	err = orm.Transaction(ctx, func(txCtx context.Context) error {
		userRow := usermodel.NewUserModel().FindMap(txCtx, map[string]any{
			"id": userID, "status": usermodel.UserStatusEnabled,
		})
		if len(userRow) == 0 {
			return NewAuthRequiredError("用户不存在或已停用")
		}
		levelRow := usermodel.NewIdentityLevelModel().FindMap(txCtx, map[string]any{
			"id": request.LevelID, "status": identityStatusEnabled, "upgrade_method": levelUpgradePay,
		})
		if len(levelRow) == 0 {
			return fmt.Errorf("订阅方案不存在或已停用")
		}
		identityID := util.ToUint64(levelRow["identity_id"])
		identityRow := usermodel.NewIdentityModel().FindMap(txCtx, map[string]any{
			"id": identityID, "status": identityStatusEnabled,
		})
		if len(identityRow) == 0 {
			return fmt.Errorf("订阅身份不存在或已停用")
		}
		pointConfigID := util.ToUint64(identityRow["purchase_point_id"])
		pointRow := usermodel.NewPointConfigModel().FindMap(txCtx, map[string]any{"id": pointConfigID})
		if len(pointRow) == 0 {
			return fmt.Errorf("订阅积分配置不存在")
		}
		currentRow := usermodel.NewUserIdentityModel().FindMap(txCtx, map[string]any{
			"user_id": userID, "identity_id": identityID,
		})
		currentLevelRow := map[string]any{}
		if currentLevelID := util.ToUint64(currentRow["level_id"]); currentLevelID > 0 {
			currentLevelRow = usermodel.NewIdentityLevelModel().FindMap(txCtx, map[string]any{"id": currentLevelID})
		}
		totalPoints := subscriptionPointPrice(levelRow, currentLevelRow, currentRow, now)
		if err := validatePointChangeAmount(totalPoints, "level_id"); err != nil {
			return err
		}
		balance := availableUserPointBalance(txCtx, userID, pointConfigID, now)
		balancePoints := totalPoints
		if balancePoints > balance {
			balancePoints = balance
		}
		rechargePoints := totalPoints - balancePoints
		payAmountMicros := int64(0)
		if rechargePoints > 0 {
			payAmountMicros, err = pointAmountToMoneyMicros(rechargePoints, util.ToIntDefault(pointRow["exchange_rate"], 0))
			if err != nil {
				return err
			}
		}
		action := identityCheckoutAction(currentRow, request.LevelID)
		levelForPeriod := clonePointPayload([]any{levelRow})
		levelForPeriod["duration_type"] = levelDurationRenew
		_, targetExpiredAt := userIdentityPeriod(now, levelForPeriod, currentRow)
		point := pointConfigSnapshot(pointRow)
		holdKey := ""
		if balancePoints > 0 {
			holdKey = "identity-order:" + orderNo + ":hold"
		}
		status := usermodel.AccountOrderStatusPendingPayment
		record := map[string]any{
			"order_no": orderNo, "request_id": requestID,
			"user_id": userID, "user_name": strings.TrimSpace(util.ToString(userRow["name"])),
			"identity_id": identityID, "identity_name": strings.TrimSpace(util.ToString(identityRow["name"])),
			"level_id": request.LevelID, "level_name": strings.TrimSpace(util.ToString(levelRow["name"])), "level": util.ToIntDefault(levelRow["level"], 0),
			"action":          action,
			"point_config_id": pointConfigID, "point_name": point.name, "point_symbol": point.symbol, "point_symbol_position": point.symbolPosition,
			"total_points": totalPoints, "balance_points": balancePoints, "recharge_points": rechargePoints,
			"pay_amount_micros": payAmountMicros, "currency": accountCurrencyCNY, "hold_business_key": holdKey,
			"status": status, "target_expired_at": targetExpiredAt, "created_at": now,
		}
		if previousExpiredAt := normalizeUserIdentityTime(currentRow["expired_at"]); !previousExpiredAt.IsZero() {
			record["previous_expired_at"] = previousExpiredAt
		}
		if rechargePoints == 0 {
			record["status"] = usermodel.AccountOrderStatusPaid
			record["paid_at"] = now
		}
		orderID, err = insertIdentityOrder(txCtx, record)
		if err != nil {
			return err
		}
		if balancePoints > 0 {
			_, err = ReservePoints(txCtx, PointReserveRequest{
				BusinessKey: holdKey, BusinessType: "identity_order",
				UserID: userID, PointConfigID: pointConfigID, Amount: balancePoints,
				Remark: "订阅订单 " + orderNo + " 积分预占", ExpiresAt: now.Add(24 * time.Hour),
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if isUniqueConflictError(err) {
			if existing := usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{
				"user_id": userID, "request_id": requestID,
			}); len(existing) > 0 {
				return accountOrderResponse(AccountOrderTypeIdentity, existing), nil
			}
		}
		return nil, err
	}
	row := usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{"id": orderID})
	if util.ToIntDefault(row["recharge_points"], 0) == 0 {
		if err := fulfillIdentityOrder(ctx, orderNo); err != nil {
			return nil, err
		}
	} else if err := startIdentityOrderPayment(ctx, row); err != nil {
		return nil, err
	}
	return accountOrderResponse(AccountOrderTypeIdentity, usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{"id": orderID})), nil
}

func insertIdentityOrder(ctx context.Context, record map[string]any) (id uint64, err error) {
	return insertAccountOrder("订阅订单", func() any {
		return usermodel.NewIdentityOrderModel().Insert(ctx, record)
	})
}

func subscriptionPointPrice(levelRow map[string]any, currentLevelRow map[string]any, currentRow map[string]any, now time.Time) int {
	targetPrice := util.ToIntDefault(levelRow["pay_amount"], 0)
	if targetPrice <= 0 || len(currentLevelRow) == 0 || util.ToUint64(currentLevelRow["id"]) == util.ToUint64(levelRow["id"]) {
		return targetPrice
	}
	if normalizeUserStatus(currentRow["status"]) != identityStatusEnabled || !normalizeUserIdentityTime(currentRow["expired_at"]).After(now) {
		return targetPrice
	}
	if util.ToIntDefault(levelRow["pay_type"], levelPayFull) != levelPayDiff {
		return targetPrice
	}
	currentPrice := util.ToIntDefault(currentLevelRow["pay_amount"], 0)
	if targetPrice > currentPrice {
		return targetPrice - currentPrice
	}
	return targetPrice
}

func identityCheckoutAction(currentRow map[string]any, levelID uint64) string {
	if util.ToUint64(currentRow["id"]) == 0 {
		return identityOrderActionSubscribe
	}
	if util.ToUint64(currentRow["level_id"]) == levelID {
		return identityOrderActionRenew
	}
	return identityOrderActionSwitch
}

func availableUserPointBalance(ctx context.Context, userID uint64, pointConfigID uint64, now time.Time) int {
	row := usermodel.NewUserPointModel().FindMap(ctx, map[string]any{
		"user_id": userID, "point_config_id": pointConfigID,
	})
	if len(row) == 0 {
		return 0
	}
	available := util.ToIntDefault(row["balance"], 0) - activePointHoldAmount(ctx, util.ToUint64(row["id"]), 0, now)
	if available < 0 {
		return 0
	}
	return available
}

func startIdentityOrderPayment(ctx context.Context, row map[string]any) error {
	status := strings.TrimSpace(util.ToString(row["status"]))
	if !accountOrderCanRetry(status) {
		if status == usermodel.AccountOrderStatusPaying || status == usermodel.AccountOrderStatusCompleted {
			return nil
		}
		return fmt.Errorf("当前订阅订单不能发起支付")
	}
	if util.ToIntDefault(row["balance_points"], 0) > 0 {
		hold, ok := FindPointHold(ctx, strings.TrimSpace(util.ToString(row["hold_business_key"])))
		if !ok || hold.Status != usermodel.PointHoldStatusActive {
			return fmt.Errorf("订单积分预占已失效，请取消后重新下单")
		}
	}
	orderID := util.ToUint64(row["id"])
	updated := usermodel.NewIdentityOrderModel().Update(ctx, map[string]any{
		"id": orderID, "status": status,
	}, map[string]any{
		"status": usermodel.AccountOrderStatusPaying, "error_message": "",
	}, false)
	if updated == 0 {
		return fmt.Errorf("订阅订单状态已变化")
	}
	result, err := createOrderPayment(ctx, PaymentCreateRequest{
		OrderType:    AccountOrderTypeIdentity,
		OrderNo:      strings.TrimSpace(util.ToString(row["order_no"])),
		Subject:      strings.TrimSpace(util.ToString(row["identity_name"])) + " · " + strings.TrimSpace(util.ToString(row["level_name"])),
		AmountMicros: util.ToInt64(row["pay_amount_micros"]),
		Currency:     strings.TrimSpace(util.ToString(row["currency"])),
	})
	if err != nil {
		usermodel.NewIdentityOrderModel().Update(ctx, map[string]any{
			"id": orderID, "status": usermodel.AccountOrderStatusPaying,
		}, map[string]any{
			"status": usermodel.AccountOrderStatusFailed, "error_message": err.Error(),
		}, false)
		return err
	}
	usermodel.NewIdentityOrderModel().Update(ctx, map[string]any{
		"id": orderID, "status": usermodel.AccountOrderStatusPaying,
	}, map[string]any{
		"payment_no": result.PaymentNo, "payment_url": result.PaymentURL,
	}, false)
	return nil
}

func CompleteIdentityPayment(ctx context.Context, orderNo string, paymentNo string) (map[string]any, error) {
	orderNo = strings.TrimSpace(orderNo)
	row := usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{"order_no": orderNo})
	if len(row) == 0 {
		return nil, fmt.Errorf("订阅订单不存在")
	}
	if strings.TrimSpace(util.ToString(row["status"])) == usermodel.AccountOrderStatusCompleted {
		return accountOrderResponse(AccountOrderTypeIdentity, row), nil
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
	updated := usermodel.NewIdentityOrderModel().Update(ctx, map[string]any{
		"id":     util.ToUint64(row["id"]),
		"status": []any{usermodel.AccountOrderStatusPendingPayment, usermodel.AccountOrderStatusPaying, usermodel.AccountOrderStatusFailed},
	}, map[string]any{
		"status": usermodel.AccountOrderStatusPaid, "payment_no": paymentNo, "error_message": "", "paid_at": now,
	}, false)
	if updated == 0 {
		latest := usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{"id": util.ToUint64(row["id"])})
		latestStatus := strings.TrimSpace(util.ToString(latest["status"]))
		if latestStatus == usermodel.AccountOrderStatusCompleted {
			return accountOrderResponse(AccountOrderTypeIdentity, latest), nil
		}
		if latestStatus != usermodel.AccountOrderStatusPaid {
			return nil, fmt.Errorf("订阅订单状态已变化")
		}
	}
	if err := fulfillIdentityOrder(ctx, orderNo); err != nil {
		latest := usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{"order_no": orderNo})
		if strings.TrimSpace(util.ToString(latest["status"])) == usermodel.AccountOrderStatusCompleted {
			return accountOrderResponse(AccountOrderTypeIdentity, latest), nil
		}
		return nil, err
	}
	return accountOrderResponse(AccountOrderTypeIdentity, usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{"order_no": orderNo})), nil
}

func fulfillIdentityOrder(ctx context.Context, orderNo string) error {
	return orm.Transaction(ctx, func(txCtx context.Context) error {
		model := usermodel.NewIdentityOrderModel()
		row := model.FindMap(txCtx, map[string]any{"order_no": strings.TrimSpace(orderNo)})
		if len(row) == 0 {
			return fmt.Errorf("订阅订单不存在")
		}
		status := strings.TrimSpace(util.ToString(row["status"]))
		if status == usermodel.AccountOrderStatusCompleted {
			return nil
		}
		if status != usermodel.AccountOrderStatusPaid {
			return fmt.Errorf("订阅订单尚未支付")
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
			return fmt.Errorf("订阅订单正在处理中")
		}
		userID := util.ToUint64(row["user_id"])
		pointConfigID := util.ToUint64(row["point_config_id"])
		rechargePoints := util.ToIntDefault(row["recharge_points"], 0)
		if rechargePoints > 0 {
			if _, err := adjustUserPointsByOperation(txCtx, "identity-order:"+orderNo+":recharge", pointAdjustRequest{
				userID: userID, pointConfigID: pointConfigID, changeType: pointChangeIncrease,
				source: pointSourcePurchase, amount: rechargePoints,
				remark: "订阅订单 " + orderNo + " 补充积分", createdAt: time.Now(),
			}); err != nil {
				return err
			}
		}
		balancePoints := util.ToIntDefault(row["balance_points"], 0)
		if balancePoints > 0 {
			settled, err := SettlePoints(txCtx, PointSettleRequest{
				BusinessKey: strings.TrimSpace(util.ToString(row["hold_business_key"])),
				Amount:      balancePoints, Remark: "订阅订单 " + orderNo + " 支付积分",
			})
			if err != nil {
				return err
			}
			if settled.SettledAmount != balancePoints {
				return fmt.Errorf("订阅订单积分结算不完整")
			}
		}
		if rechargePoints > 0 {
			if _, err := adjustUserPointsByOperation(txCtx, "identity-order:"+orderNo+":consume", pointAdjustRequest{
				userID: userID, pointConfigID: pointConfigID, changeType: pointChangeConsume,
				source: pointSourcePurchase, amount: rechargePoints,
				remark: "订阅订单 " + orderNo + " 支付积分", createdAt: time.Now(),
				skipGrantConsumption: true,
			}); err != nil {
				return err
			}
		}
		userRow := usermodel.NewUserModel().FindMap(txCtx, map[string]any{"id": userID})
		identityRow := usermodel.NewIdentityModel().FindMap(txCtx, map[string]any{"id": util.ToUint64(row["identity_id"])})
		levelRow := usermodel.NewIdentityLevelModel().FindMap(txCtx, map[string]any{"id": util.ToUint64(row["level_id"])})
		if len(userRow) == 0 || len(identityRow) == 0 || len(levelRow) == 0 {
			return fmt.Errorf("订阅履约所需配置不存在")
		}
		levelRow = clonePointPayload([]any{levelRow})
		levelRow["duration_type"] = levelDurationRenew
		payload := userIdentityPayload{
			userID: userID, userRow: userRow,
			identityID: util.ToUint64(identityRow["id"]), identityRow: identityRow,
			levelID: util.ToUint64(levelRow["id"]), levelRow: levelRow,
			status: identityStatusEnabled, remark: "订阅订单 " + orderNo,
		}
		now := time.Now()
		saved, err := saveUserIdentity(txCtx, payload, now)
		if err != nil {
			return err
		}
		userIdentityRow := payload.userIdentityRecord(saved.cardNo, saved.expiredAt)
		userIdentityRow["id"] = saved.id
		if err := issueDueIdentityBenefitForUserIdentity(txCtx, userIdentityRow, now); err != nil {
			return err
		}
		if model.Update(txCtx, map[string]any{
			"id": orderID, "status": usermodel.AccountOrderStatusFulfilling,
		}, map[string]any{
			"status": usermodel.AccountOrderStatusCompleted, "target_expired_at": saved.expiredAt,
			"fulfilled_at": now, "error_message": "",
		}, false) == 0 {
			return fmt.Errorf("订阅订单完成状态保存失败")
		}
		return nil
	})
}
