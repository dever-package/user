package service

import (
	"context"
	"strings"
	"time"

	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

type AccountService struct{}

type AccountPageRequest struct {
	Cursor        string
	Limit         int
	Type          string
	PointConfigID uint64
}

func (AccountService) Overview(ctx context.Context) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	userRow := usermodel.NewUserModel().FindMap(ctx, map[string]any{
		"id": userID, "status": usermodel.UserStatusEnabled,
	})
	if len(userRow) == 0 {
		return nil, NewAuthRequiredError("用户不存在或已停用")
	}
	now := time.Now()
	pointConfigRows := usermodel.NewPointConfigModel().SelectMap(ctx, nil, map[string]any{
		"order": "sort asc,id asc",
	})
	pointAccountRows := usermodel.NewUserPointModel().SelectMap(ctx, map[string]any{"user_id": userID}, map[string]any{
		"order": "point_config_id asc,id asc",
	})
	subscriptionState := loadAccountSubscriptions(ctx, userID, now)
	return map[string]any{
		"user": map[string]any{
			"id": userID, "name": strings.TrimSpace(util.ToString(userRow["name"])),
			"mobile": strings.TrimSpace(util.ToString(userRow["mobile"])),
		},
		"point_accounts": accountPointBalances(ctx, pointConfigRows, pointAccountRows, now),
		"subscriptions":  subscriptionState.items,
		"catalog":        loadAccountCatalog(ctx, subscriptionState, now),
		"point_packages": loadAccountPointPackages(ctx),
	}, nil
}

type accountSubscriptionState struct {
	rows             []map[string]any
	activeByIdentity map[uint64]map[string]any
	items            []map[string]any
}

func loadAccountSubscriptions(ctx context.Context, userID uint64, now time.Time) accountSubscriptionState {
	userIdentityRows := usermodel.NewUserIdentityModel().SelectMap(ctx, map[string]any{
		"user_id": userID, "status": identityStatusEnabled,
	}, map[string]any{"order": "identity_id asc,id asc"})
	identitiesByID := map[uint64]map[string]any{}
	if identityIDs := collectAccountIDs(userIdentityRows, "identity_id"); len(identityIDs) > 0 {
		identitiesByID = accountRowsByID(usermodel.NewIdentityModel().SelectMap(ctx, map[string]any{
			"id": identityIDs,
		}, nil))
	}
	activeIdentityByID := make(map[uint64]map[string]any, len(userIdentityRows))
	subscriptions := make([]map[string]any, 0, len(userIdentityRows))
	for _, row := range userIdentityRows {
		expiresAt := normalizeUserIdentityTime(row["expired_at"])
		if expiresAt.IsZero() || !expiresAt.After(now) {
			continue
		}
		identityID := util.ToUint64(row["identity_id"])
		pointConfigID := util.ToUint64(identitiesByID[identityID]["purchase_point_id"])
		activeIdentityByID[identityID] = row
		subscriptions = append(subscriptions, map[string]any{
			"id": util.ToUint64(row["id"]), "identity_id": identityID,
			"point_config_id": pointConfigID,
			"identity_name":   strings.TrimSpace(util.ToString(row["identity_name"])),
			"level_id":        util.ToUint64(row["level_id"]), "level_name": strings.TrimSpace(util.ToString(row["level_name"])),
			"level": util.ToIntDefault(row["level"], 0), "card_no": strings.TrimSpace(util.ToString(row["card_no"])),
			"expired_at": accountOrderTime(expiresAt),
		})
	}
	return accountSubscriptionState{
		rows:             userIdentityRows,
		activeByIdentity: activeIdentityByID,
		items:            subscriptions,
	}
}

func loadAccountCatalog(ctx context.Context, subscriptions accountSubscriptionState, now time.Time) []map[string]any {
	identities := usermodel.NewIdentityModel().SelectMap(ctx, map[string]any{"status": identityStatusEnabled}, map[string]any{
		"order": "sort asc,id asc",
	})
	levels := usermodel.NewIdentityLevelModel().SelectMap(ctx, map[string]any{
		"status": identityStatusEnabled, "upgrade_method": levelUpgradePay,
	}, map[string]any{"order": "identity_id asc,sort asc,level asc,id asc"})
	levelIDs := collectAccountIDs(levels, "id")
	descriptionsByLevel := map[uint64][]map[string]any{}
	periodicByLevel := map[uint64][]map[string]any{}
	billingByLevel := map[uint64][]map[string]any{}
	if len(levelIDs) > 0 {
		descriptionsByLevel = groupAccountRows(usermodel.NewIdentityBenefitDescriptionModel().SelectMap(ctx, map[string]any{
			"level_id": levelIDs,
		}, map[string]any{"order": "level_id asc,sort asc,id asc"}), "level_id")
		periodicByLevel = groupAccountRows(usermodel.NewIdentityBenefitModel().SelectMap(ctx, map[string]any{
			"level_id": levelIDs, "status": identityStatusEnabled,
		}, map[string]any{"order": "level_id asc,sort asc,id asc"}), "level_id")
		billingByLevel = groupAccountRows(usermodel.NewIdentityBillingBenefitModel().SelectMap(ctx, map[string]any{
			"level_id": levelIDs, "status": identityStatusEnabled,
		}, map[string]any{"order": "level_id asc,sort asc,id asc"}), "level_id")
	}
	levelsByIdentity := groupAccountRows(levels, "identity_id")
	levelLookupIDs := append([]any{}, levelIDs...)
	knownLevelIDs := map[uint64]bool{}
	for _, value := range levelLookupIDs {
		knownLevelIDs[util.ToUint64(value)] = true
	}
	for _, row := range subscriptions.rows {
		levelID := util.ToUint64(row["level_id"])
		if levelID > 0 && !knownLevelIDs[levelID] {
			levelLookupIDs = append(levelLookupIDs, levelID)
			knownLevelIDs[levelID] = true
		}
	}
	levelRowsByID := map[uint64]map[string]any{}
	if len(levelLookupIDs) > 0 {
		levelRowsByID = accountRowsByID(usermodel.NewIdentityLevelModel().SelectMap(ctx, map[string]any{"id": levelLookupIDs}, nil))
	}
	pointConfigIDs := collectAccountIDs(identities, "purchase_point_id")
	pointConfigs := map[uint64]map[string]any{}
	if len(pointConfigIDs) > 0 {
		pointConfigs = accountRowsByID(usermodel.NewPointConfigModel().SelectMap(ctx, map[string]any{"id": pointConfigIDs}, nil))
	}
	catalog := make([]map[string]any, 0, len(identities))
	for _, identityRow := range identities {
		identityID := util.ToUint64(identityRow["id"])
		identityLevels := levelsByIdentity[identityID]
		if len(identityLevels) == 0 {
			continue
		}
		pointRow := pointConfigs[util.ToUint64(identityRow["purchase_point_id"])]
		if len(pointRow) == 0 {
			continue
		}
		currentRow := subscriptions.activeByIdentity[identityID]
		currentLevelRow := levelRowsByID[util.ToUint64(currentRow["level_id"])]
		levelItems := make([]map[string]any, 0, len(identityLevels))
		for _, levelRow := range identityLevels {
			levelID := util.ToUint64(levelRow["id"])
			basePoints := util.ToIntDefault(levelRow["pay_amount"], 0)
			checkoutPoints := subscriptionPointPrice(levelRow, currentLevelRow, currentRow, now)
			money, _ := pointAmountToMoneyMicros(checkoutPoints, util.ToIntDefault(pointRow["exchange_rate"], 0))
			levelItems = append(levelItems, map[string]any{
				"id": levelID, "name": strings.TrimSpace(util.ToString(levelRow["name"])),
				"level": util.ToIntDefault(levelRow["level"], 0), "duration_days": util.ToIntDefault(levelRow["duration_days"], 0),
				"pay_type": util.ToIntDefault(levelRow["pay_type"], 0), "base_points": basePoints,
				"checkout_points": checkoutPoints, "pay_amount_micros": money,
				"benefit_descriptions": accountBenefitDescriptions(descriptionsByLevel[levelID]),
				"periodic_benefits":    accountPeriodicBenefits(periodicByLevel[levelID]),
				"billing_benefits":     accountBillingBenefits(billingByLevel[levelID]),
			})
		}
		catalog = append(catalog, map[string]any{
			"id": identityID, "name": strings.TrimSpace(util.ToString(identityRow["name"])),
			"point_config": accountPointConfig(pointRow), "levels": levelItems,
			"current_level_id":   util.ToUint64(currentRow["level_id"]),
			"current_expired_at": accountOrderTime(currentRow["expired_at"]),
		})
	}
	return catalog
}

func loadAccountPointPackages(ctx context.Context) []map[string]any {
	packageRows := usermodel.NewPointPackageModel().SelectMap(ctx, map[string]any{"status": identityStatusEnabled}, map[string]any{
		"order": "sort asc,id asc",
	})
	packagePointIDs := collectAccountIDs(packageRows, "point_config_id")
	packagePointConfigs := map[uint64]map[string]any{}
	if len(packagePointIDs) > 0 {
		packagePointConfigs = accountRowsByID(usermodel.NewPointConfigModel().SelectMap(ctx, map[string]any{"id": packagePointIDs}, nil))
	}
	packages := make([]map[string]any, 0, len(packageRows))
	for _, row := range packageRows {
		pointRow := packagePointConfigs[util.ToUint64(row["point_config_id"])]
		pointAmount := util.ToIntDefault(row["point_amount"], 0)
		money, moneyErr := pointAmountToMoneyMicros(pointAmount, util.ToIntDefault(pointRow["exchange_rate"], 0))
		if moneyErr != nil {
			continue
		}
		packages = append(packages, map[string]any{
			"id": util.ToUint64(row["id"]), "name": strings.TrimSpace(util.ToString(row["name"])),
			"point_config": accountPointConfig(pointRow), "point_amount": pointAmount,
			"bonus_amount": util.ToIntDefault(row["bonus_amount"], 0), "pay_amount_micros": money,
		})
	}
	return packages
}

func accountPointBalances(
	ctx context.Context,
	pointConfigRows []map[string]any,
	pointAccountRows []map[string]any,
	now time.Time,
) []map[string]any {
	accountsByPointConfig := make(map[uint64]map[string]any, len(pointAccountRows))
	for _, row := range pointAccountRows {
		if pointConfigID := util.ToUint64(row["point_config_id"]); pointConfigID > 0 {
			accountsByPointConfig[pointConfigID] = row
		}
	}

	result := make([]map[string]any, 0, len(pointConfigRows))
	for _, pointConfigRow := range pointConfigRows {
		pointConfigID := util.ToUint64(pointConfigRow["id"])
		if pointConfigID == 0 {
			continue
		}
		accountRow := accountsByPointConfig[pointConfigID]
		balance := util.ToIntDefault(accountRow["balance"], 0)
		available := balance - activePointHoldAmount(ctx, util.ToUint64(accountRow["id"]), 0, now)
		if available < 0 {
			available = 0
		}
		result = append(result, map[string]any{
			"id": util.ToUint64(accountRow["id"]), "point_config_id": pointConfigID,
			"name": strings.TrimSpace(util.ToString(pointConfigRow["name"])), "symbol": strings.TrimSpace(util.ToString(pointConfigRow["symbol"])),
			"symbol_position": util.ToIntDefault(pointConfigRow["symbol_position"], 2),
			"balance":         balance, "available_balance": available,
		})
	}
	return result
}

func accountPointConfig(row map[string]any) map[string]any {
	return map[string]any{
		"id": util.ToUint64(row["id"]), "name": strings.TrimSpace(util.ToString(row["name"])),
		"exchange_rate": util.ToIntDefault(row["exchange_rate"], 0), "symbol": strings.TrimSpace(util.ToString(row["symbol"])),
		"symbol_position": util.ToIntDefault(row["symbol_position"], 2),
	}
}

func accountPeriodicBenefits(rows []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{
			"point_name": strings.TrimSpace(util.ToString(row["point_name"])), "point_amount": util.ToIntDefault(row["point_amount"], 0),
			"cycle_days": util.ToIntDefault(row["cycle_days"], 0), "limit_times": util.ToIntDefault(row["limit_times"], 0),
		})
	}
	return result
}

func accountBenefitDescriptions(rows []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		content := strings.TrimSpace(util.ToString(row["content"]))
		if content == "" {
			continue
		}
		result = append(result, map[string]any{
			"icon": strings.TrimSpace(util.ToString(row["icon"])),
			"text": content,
		})
	}
	return result
}

func accountBillingBenefits(rows []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{
			"scope": strings.TrimSpace(util.ToString(row["scope"])), "sale_ratio": strings.TrimSpace(util.ToString(row["sale_ratio"])),
		})
	}
	return result
}

func collectAccountIDs(rows []map[string]any, field string) []any {
	result := make([]any, 0, len(rows))
	seen := map[uint64]bool{}
	for _, row := range rows {
		id := util.ToUint64(row[field])
		if id > 0 && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result
}

func accountRowsByID(rows []map[string]any) map[uint64]map[string]any {
	result := make(map[uint64]map[string]any, len(rows))
	for _, row := range rows {
		if id := util.ToUint64(row["id"]); id > 0 {
			result[id] = row
		}
	}
	return result
}

func groupAccountRows(rows []map[string]any, field string) map[uint64][]map[string]any {
	result := map[uint64][]map[string]any{}
	for _, row := range rows {
		id := util.ToUint64(row[field])
		if id > 0 {
			result[id] = append(result[id], row)
		}
	}
	return result
}
