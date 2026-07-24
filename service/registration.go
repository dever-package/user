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
		grants := loadRegistrationIdentityGrants(txCtx)
		if len(grants) == 0 {
			return nil
		}
		existing := registrationIdentitySet(txCtx, []uint64{userID}, registrationGrantIdentityIDs(grants))[userID]
		return grantRegistrationIdentitiesWithConfig(txCtx, userID, userRow, now, grants, existing, func(userIdentityRow map[string]any) error {
			return issueDueIdentityBenefitForUserIdentity(txCtx, userIdentityRow, now)
		})
	})
}

func backfillRegistrationIdentities(ctx context.Context, now time.Time) error {
	var resultErr error
	grants := loadRegistrationIdentityGrants(ctx)
	if len(grants) == 0 {
		return nil
	}
	identityIDs := registrationGrantIdentityIDs(grants)
	for page := 1; ; page++ {
		rows := usermodel.NewUserModel().SelectMap(ctx, map[string]any{
			"status": usermodel.UserStatusEnabled,
		}, map[string]any{
			"order": "id asc", "page": page, "pageSize": registrationBackfillPageSize,
		})
		userIDs := make([]uint64, 0, len(rows))
		for _, row := range rows {
			if userID := util.ToUint64(row["id"]); userID > 0 {
				userIDs = append(userIDs, userID)
			}
		}
		existingByUser := registrationIdentitySet(ctx, userIDs, identityIDs)
		for _, row := range rows {
			userID := util.ToUint64(row["id"])
			if userID == 0 {
				continue
			}
			if err := orm.Transaction(ctx, func(txCtx context.Context) error {
				return grantRegistrationIdentitiesWithConfig(txCtx, userID, row, now, grants, existingByUser[userID], nil)
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
