package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

const registrationBackfillPageSize = 200

func initializeRegistrationBenefits(ctx context.Context, userID uint64, userRow map[string]any, now time.Time) error {
	if userID == 0 || len(userRow) == 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	return orm.Transaction(ctx, func(txCtx context.Context) error {
		if err := grantRegistrationIdentities(txCtx, userID, userRow, now); err != nil {
			return err
		}
		return issueDueIdentityBenefitsForUser(txCtx, userID, now)
	})
}

func backfillRegistrationIdentities(ctx context.Context, now time.Time) error {
	var resultErr error
	for page := 1; ; page++ {
		rows := usermodel.NewUserModel().SelectMap(ctx, map[string]any{
			"status": usermodel.UserStatusEnabled,
		}, map[string]any{
			"order": "id asc", "page": page, "pageSize": registrationBackfillPageSize,
		})
		for _, row := range rows {
			userID := util.ToUint64(row["id"])
			if userID == 0 {
				continue
			}
			if err := orm.Transaction(ctx, func(txCtx context.Context) error {
				return grantRegistrationIdentities(txCtx, userID, row, now)
			}); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("用户 %d 补授注册身份失败: %w", userID, err))
			}
		}
		if len(rows) < registrationBackfillPageSize {
			break
		}
	}
	return resultErr
}
