package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

func (AccountService) OrderStatus(ctx context.Context, orderType string, orderNo string) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return ownedAccountOrder(ctx, userID, orderType, orderNo)
}

func (AccountService) RetryOrder(ctx context.Context, orderType string, orderNo string) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	orderType = normalizeAccountOrderType(orderType)
	row, err := ownedAccountOrderRow(ctx, userID, orderType, orderNo)
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(util.ToString(row["status"]))
	if status == usermodel.AccountOrderStatusCompleted || status == usermodel.AccountOrderStatusPaying {
		return accountOrderResponse(orderType, row), nil
	}
	if status == usermodel.AccountOrderStatusPaid {
		if orderType == AccountOrderTypeIdentity {
			err = fulfillIdentityOrder(ctx, orderNo)
		} else {
			err = fulfillPointRechargeOrder(ctx, orderNo)
		}
	} else if accountOrderCanRetry(status) {
		if orderType == AccountOrderTypeIdentity {
			err = startIdentityOrderPayment(ctx, row)
		} else {
			err = startPointOrderPayment(ctx, row)
		}
	} else {
		err = fmt.Errorf("当前订单不能重试")
	}
	if err != nil {
		return nil, err
	}
	return ownedAccountOrder(ctx, userID, orderType, orderNo)
}

func (AccountService) CancelOrder(ctx context.Context, orderType string, orderNo string) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	orderType = normalizeAccountOrderType(orderType)
	row, err := ownedAccountOrderRow(ctx, userID, orderType, orderNo)
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(util.ToString(row["status"]))
	if status == usermodel.AccountOrderStatusCanceled {
		return accountOrderResponse(orderType, row), nil
	}
	if !accountOrderCanCancel(status) {
		return nil, fmt.Errorf("当前订单不能取消")
	}
	if err := cancelOrderPayment(ctx, strings.TrimSpace(util.ToString(row["payment_no"]))); err != nil {
		return nil, err
	}
	err = orm.Transaction(ctx, func(txCtx context.Context) error {
		if orderType == AccountOrderTypeIdentity {
			return cancelIdentityOrder(txCtx, row, status)
		}
		if usermodel.NewPointRechargeOrderModel().Update(txCtx, map[string]any{
			"id": util.ToUint64(row["id"]), "status": status,
		}, map[string]any{"status": usermodel.AccountOrderStatusCanceled, "payment_url": "", "error_message": ""}, false) == 0 {
			return fmt.Errorf("订单状态已变化")
		}
		return nil
	})
	if err != nil {
		latest, lookupErr := ownedAccountOrderRow(ctx, userID, orderType, orderNo)
		if lookupErr == nil && strings.TrimSpace(util.ToString(latest["status"])) == usermodel.AccountOrderStatusCanceled {
			return accountOrderResponse(orderType, latest), nil
		}
		return nil, err
	}
	return ownedAccountOrder(ctx, userID, orderType, orderNo)
}

func cancelIdentityOrder(ctx context.Context, row map[string]any, status string) error {
	holdKey := strings.TrimSpace(util.ToString(row["hold_business_key"]))
	if holdKey != "" {
		if _, err := ReleasePoints(ctx, PointReleaseRequest{BusinessKey: holdKey, Remark: "订阅订单已取消"}); err != nil {
			return err
		}
	}
	if usermodel.NewIdentityOrderModel().Update(ctx, map[string]any{
		"id": util.ToUint64(row["id"]), "status": status,
	}, map[string]any{"status": usermodel.AccountOrderStatusCanceled, "payment_url": "", "error_message": ""}, false) == 0 {
		return fmt.Errorf("订单状态已变化")
	}
	return nil
}

func ownedAccountOrder(ctx context.Context, userID uint64, orderType string, orderNo string) (map[string]any, error) {
	orderType = normalizeAccountOrderType(orderType)
	row, err := ownedAccountOrderRow(ctx, userID, orderType, orderNo)
	if err != nil {
		return nil, err
	}
	return accountOrderResponse(orderType, row), nil
}

func ownedAccountOrderRow(ctx context.Context, userID uint64, orderType string, orderNo string) (map[string]any, error) {
	orderNo = strings.TrimSpace(orderNo)
	if userID == 0 || orderType == "" || orderNo == "" {
		return nil, fmt.Errorf("订单参数不正确")
	}
	var row map[string]any
	if orderType == AccountOrderTypeIdentity {
		row = usermodel.NewIdentityOrderModel().FindMap(ctx, map[string]any{"user_id": userID, "order_no": orderNo})
	} else {
		row = usermodel.NewPointRechargeOrderModel().FindMap(ctx, map[string]any{"user_id": userID, "order_no": orderNo})
	}
	if len(row) == 0 {
		return nil, fmt.Errorf("订单不存在")
	}
	return row, nil
}
