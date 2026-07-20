package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

type identityBenefitIssueStats struct {
	BenefitCount  int `json:"benefit_count"`
	UserCount     int `json:"user_count"`
	IssuedCount   int `json:"issued_count"`
	SkippedCount  int `json:"skipped_count"`
	ClearedCount  int `json:"cleared_count"`
	ClearedAmount int `json:"cleared_amount"`
	ErrorCount    int `json:"error_count"`
}

type identityBenefitCycle struct {
	startAt time.Time
	endAt   time.Time
}

func (BenefitService) IssueDueIdentityBenefits(ctx context.Context, now time.Time) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}

	stats := identityBenefitIssueStats{}
	var resultErr error
	if err := backfillRegistrationIdentities(ctx, now); err != nil {
		stats.ErrorCount++
		resultErr = errors.Join(resultErr, err)
	}
	benefitRows := usermodel.NewIdentityBenefitModel().SelectMap(ctx, map[string]any{
		"benefit_type": benefitTypeRewardPoint,
		"status":       identityStatusEnabled,
	}, map[string]any{
		"order": "identity_id asc,level asc,sort asc,id asc",
	})
	for _, benefitRow := range benefitRows {
		if !isRunnableIdentityBenefit(benefitRow) {
			stats.SkippedCount++
			continue
		}
		stats.BenefitCount++
		for page := 1; ; page++ {
			userIdentityRows := activeUserIdentityRowsForBenefit(ctx, benefitRow, now, 0, page, 200)
			for _, userIdentityRow := range userIdentityRows {
				stats.UserCount++
				result, err := issueIdentityBenefitToUser(ctx, benefitRow, userIdentityRow, now)
				if err != nil {
					stats.ErrorCount++
					resultErr = errors.Join(resultErr, fmt.Errorf("用户身份 %d 发放权益 %d 失败: %w",
						util.ToUint64(userIdentityRow["id"]), util.ToUint64(benefitRow["id"]), err))
					continue
				}
				if result.issued {
					stats.IssuedCount++
				} else {
					stats.SkippedCount++
				}
				if result.clearedAmount > 0 {
					stats.ClearedCount++
					stats.ClearedAmount += result.clearedAmount
				}
			}
			if len(userIdentityRows) < 200 {
				break
			}
		}
	}
	return stats.toMap(now, resultErr), resultErr
}

func issueDueIdentityBenefitsForUser(ctx context.Context, userID uint64, now time.Time) error {
	benefitRows := usermodel.NewIdentityBenefitModel().SelectMap(ctx, map[string]any{
		"benefit_type": benefitTypeRewardPoint,
		"status":       identityStatusEnabled,
	}, map[string]any{"order": "identity_id asc,level asc,sort asc,id asc"})
	for _, benefitRow := range benefitRows {
		if !isRunnableIdentityBenefit(benefitRow) {
			continue
		}
		rows := activeUserIdentityRowsForBenefit(ctx, benefitRow, now, userID, 1, 100)
		for _, row := range rows {
			if _, err := issueIdentityBenefitToUser(ctx, benefitRow, row, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (stats identityBenefitIssueStats) toMap(now time.Time, runErr error) map[string]any {
	result := map[string]any{
		"benefit_count":  stats.BenefitCount,
		"user_count":     stats.UserCount,
		"issued_count":   stats.IssuedCount,
		"skipped_count":  stats.SkippedCount,
		"cleared_count":  stats.ClearedCount,
		"cleared_amount": stats.ClearedAmount,
		"error_count":    stats.ErrorCount,
		"run_at":         now.Format(time.RFC3339),
	}
	if runErr != nil {
		result["error"] = runErr.Error()
	}
	return result
}

type identityBenefitIssueResult struct {
	issued        bool
	clearedAmount int
}

func issueIdentityBenefitToUser(ctx context.Context, benefitRow map[string]any, userIdentityRow map[string]any, now time.Time) (identityBenefitIssueResult, error) {
	result := identityBenefitIssueResult{}
	err := orm.Transaction(ctx, func(txCtx context.Context) error {
		cycle, ok := currentIdentityBenefitCycle(benefitRow, userIdentityRow, now)
		if !ok {
			return nil
		}
		grantRows := currentCycleGrantRows(txCtx, userIdentityRow, benefitRow, cycle.startAt)
		limitTimes := normalizeBenefitPositiveInt(benefitRow["limit_times"], 1)
		if len(grantRows) >= limitTimes {
			return nil
		}
		grantNo := len(grantRows) + 1
		if grantNo == 1 && normalizeBenefitClearPrevious(benefitRow["clear_previous"]) == benefitClearEnabled {
			amount, err := clearPreviousIdentityBenefitGrants(txCtx, userIdentityRow, benefitRow, cycle.startAt, now)
			if err != nil {
				return err
			}
			result.clearedAmount = amount
		}
		if err := createIdentityBenefitGrant(txCtx, benefitRow, userIdentityRow, cycle, grantNo, now); err != nil {
			return err
		}
		result.issued = true
		return nil
	})
	if err != nil {
		if isUniqueConflictError(err) {
			return identityBenefitIssueResult{}, nil
		}
		return identityBenefitIssueResult{}, err
	}
	return result, nil
}

func isRunnableIdentityBenefit(row map[string]any) bool {
	return util.ToUint64(row["id"]) > 0 &&
		util.ToUint64(row["identity_id"]) > 0 &&
		util.ToUint64(row["level_id"]) > 0 &&
		util.ToUint64(row["point_config_id"]) > 0 &&
		normalizeBenefitType(row["benefit_type"]) == benefitTypeRewardPoint &&
		normalizeUserStatus(row["status"]) == identityStatusEnabled &&
		util.ToIntDefault(row["point_amount"], 0) > 0 &&
		util.ToIntDefault(row["point_amount"], 0) <= maxPointChangeAmount &&
		normalizeBenefitPositiveInt(row["cycle_days"], 1) <= maxBenefitCycleDays &&
		normalizeBenefitPositiveInt(row["limit_times"], 1) <= maxBenefitLimitTimes
}

func activeUserIdentityRowsForBenefit(ctx context.Context, benefitRow map[string]any, now time.Time, userID uint64, page int, pageSize int) []map[string]any {
	filters := map[string]any{
		"identity_id": util.ToUint64(benefitRow["identity_id"]),
		"level_id":    util.ToUint64(benefitRow["level_id"]),
		"status":      identityStatusEnabled,
		"expires_at":  map[string]any{"gt": now},
	}
	if userID > 0 {
		filters["user_id"] = userID
	}
	return usermodel.NewUserIdentityModel().SelectMap(ctx, filters, map[string]any{
		"order": "user_id asc,id asc",
		"page":  page, "pageSize": pageSize,
	})
}

func currentIdentityBenefitCycle(benefitRow map[string]any, userIdentityRow map[string]any, now time.Time) (identityBenefitCycle, bool) {
	cycleDays := normalizeBenefitPositiveInt(benefitRow["cycle_days"], 1)
	if cycleDays <= 0 {
		return identityBenefitCycle{}, false
	}

	anchor := laterTime(
		normalizeUserIdentityTime(userIdentityRow["created_at"]),
		normalizeUserIdentityTime(benefitRow["created_at"]),
	)
	if anchor.IsZero() {
		anchor = now
	}
	if now.Before(anchor) {
		return identityBenefitCycle{}, false
	}

	cycleLength := time.Duration(cycleDays) * 24 * time.Hour
	cycleIndex := int(now.Sub(anchor) / cycleLength)
	cycleStartAt := anchor.AddDate(0, 0, cycleIndex*cycleDays)
	return identityBenefitCycle{
		startAt: cycleStartAt,
		endAt:   cycleStartAt.AddDate(0, 0, cycleDays),
	}, true
}

func laterTime(left time.Time, right time.Time) time.Time {
	if left.IsZero() {
		return right
	}
	if right.IsZero() {
		return left
	}
	if right.After(left) {
		return right
	}
	return left
}

func currentCycleGrantRows(ctx context.Context, userIdentityRow map[string]any, benefitRow map[string]any, cycleStartAt time.Time) []map[string]any {
	return usermodel.NewUserBenefitGrantModel().SelectMap(ctx, map[string]any{
		"user_identity_id":    util.ToUint64(userIdentityRow["id"]),
		"identity_benefit_id": util.ToUint64(benefitRow["id"]),
		"cycle_start_at":      cycleStartAt,
	}, map[string]any{
		"order": "grant_no asc,id asc",
	})
}

func clearPreviousIdentityBenefitGrants(ctx context.Context, userIdentityRow map[string]any, benefitRow map[string]any, currentCycleStartAt time.Time, now time.Time) (int, error) {
	grantModel := usermodel.NewUserBenefitGrantModel()
	rows := grantModel.SelectMap(ctx, map[string]any{
		"user_identity_id":    util.ToUint64(userIdentityRow["id"]),
		"identity_benefit_id": util.ToUint64(benefitRow["id"]),
		"status":              benefitGrantStatusActive,
	}, map[string]any{
		"order": "cycle_start_at asc,grant_no asc,id asc",
	})

	grantIDs := make([]uint64, 0, len(rows))
	totalRemaining := 0
	for _, row := range rows {
		grantID := util.ToUint64(row["id"])
		remainingAmount := util.ToIntDefault(row["remaining_amount"], 0)
		cycleStartAt := normalizeUserIdentityTime(row["cycle_start_at"])
		if grantID == 0 || remainingAmount <= 0 || cycleStartAt.IsZero() || !cycleStartAt.Before(currentCycleStartAt) {
			continue
		}
		grantIDs = append(grantIDs, grantID)
		totalRemaining += remainingAmount
	}
	if totalRemaining <= 0 {
		return 0, nil
	}

	clearAmount := benefitClearAmount(ctx, userIdentityRow, benefitRow, totalRemaining)
	if clearAmount <= 0 {
		markBenefitGrantsCleared(ctx, grantIDs, now)
		return 0, nil
	}

	_, err := adjustUserPoints(ctx, pointAdjustRequest{
		userID:               util.ToUint64(userIdentityRow["user_id"]),
		pointConfigID:        util.ToUint64(benefitRow["point_config_id"]),
		changeType:           pointChangeConsume,
		source:               pointSourceCron,
		amount:               clearAmount,
		remark:               formatBenefitClearRemark(benefitRow, clearAmount),
		createdAt:            now,
		skipGrantConsumption: true,
		afterUpdate: func(txCtx context.Context, _ pointAdjustState) error {
			markBenefitGrantsCleared(txCtx, grantIDs, now)
			return nil
		},
	})
	if err != nil {
		return 0, err
	}
	return clearAmount, nil
}

func benefitClearAmount(ctx context.Context, userIdentityRow map[string]any, benefitRow map[string]any, totalRemaining int) int {
	if totalRemaining <= 0 {
		return 0
	}
	userPointRow := usermodel.NewUserPointModel().FindMap(ctx, map[string]any{
		"user_id":         util.ToUint64(userIdentityRow["user_id"]),
		"point_config_id": util.ToUint64(benefitRow["point_config_id"]),
	})
	balance := util.ToIntDefault(userPointRow["balance"], 0)
	if balance <= 0 {
		return 0
	}
	if totalRemaining > balance {
		return balance
	}
	return totalRemaining
}

func markBenefitGrantsCleared(ctx context.Context, grantIDs []uint64, now time.Time) {
	if len(grantIDs) == 0 {
		return
	}
	usermodel.NewUserBenefitGrantModel().Update(ctx, map[string]any{
		"id": grantIDs,
	}, map[string]any{
		"remaining_amount": 0,
		"status":           benefitGrantStatusCleared,
		"cleared_at":       now,
	}, false)
}

func createIdentityBenefitGrant(ctx context.Context, benefitRow map[string]any, userIdentityRow map[string]any, cycle identityBenefitCycle, grantNo int, now time.Time) error {
	amount := util.ToIntDefault(benefitRow["point_amount"], 0)
	if amount <= 0 {
		return nil
	}
	_, err := adjustUserPoints(ctx, pointAdjustRequest{
		userID:        util.ToUint64(userIdentityRow["user_id"]),
		pointConfigID: util.ToUint64(benefitRow["point_config_id"]),
		changeType:    pointChangeIncrease,
		source:        pointSourceCron,
		amount:        amount,
		remark:        formatBenefitIssueRemark(benefitRow),
		createdAt:     now,
		afterUpdate: func(txCtx context.Context, _ pointAdjustState) error {
			return insertBenefitGrantRow(txCtx, benefitGrantRecord(benefitRow, userIdentityRow, cycle, grantNo, now))
		},
	})
	return err
}

func insertBenefitGrantRow(ctx context.Context, record map[string]any) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok {
				err = recoveredErr
				return
			}
			err = fmt.Errorf("%v", recovered)
		}
	}()
	grantID := util.ToUint64(usermodel.NewUserBenefitGrantModel().Insert(ctx, record))
	if grantID == 0 {
		return fmt.Errorf("用户周期权益发放记录保存失败")
	}
	return nil
}

func benefitGrantRecord(benefitRow map[string]any, userIdentityRow map[string]any, cycle identityBenefitCycle, grantNo int, now time.Time) map[string]any {
	amount := util.ToIntDefault(benefitRow["point_amount"], 0)
	return map[string]any{
		"user_identity_id":      util.ToUint64(userIdentityRow["id"]),
		"user_id":               util.ToUint64(userIdentityRow["user_id"]),
		"user_name":             strings.TrimSpace(util.ToString(userIdentityRow["user_name"])),
		"user_mobile":           strings.TrimSpace(util.ToString(userIdentityRow["user_mobile"])),
		"identity_benefit_id":   util.ToUint64(benefitRow["id"]),
		"identity_id":           util.ToUint64(benefitRow["identity_id"]),
		"identity_name":         strings.TrimSpace(util.ToString(benefitRow["identity_name"])),
		"level_id":              util.ToUint64(benefitRow["level_id"]),
		"level_name":            strings.TrimSpace(util.ToString(benefitRow["level_name"])),
		"level":                 util.ToIntDefault(benefitRow["level"], 0),
		"point_config_id":       util.ToUint64(benefitRow["point_config_id"]),
		"point_name":            strings.TrimSpace(util.ToString(benefitRow["point_name"])),
		"point_symbol":          strings.TrimSpace(util.ToString(benefitRow["point_symbol"])),
		"point_symbol_position": util.ToIntDefault(benefitRow["point_symbol_position"], 2),
		"amount":                amount,
		"remaining_amount":      amount,
		"cycle_start_at":        cycle.startAt,
		"cycle_end_at":          cycle.endAt,
		"grant_no":              grantNo,
		"status":                benefitGrantStatusActive,
		"created_at":            now,
	}
}

func formatBenefitIssueRemark(benefitRow map[string]any) string {
	identityName := strings.TrimSpace(util.ToString(benefitRow["identity_name"]))
	levelName := strings.TrimSpace(util.ToString(benefitRow["level_name"]))
	amount := util.ToString(util.ToIntDefault(benefitRow["point_amount"], 0))
	pointName := benefitPointName(benefitRow)
	return "周期权益发放：" + formatBenefitIdentityLevel(identityName, levelName) + "，发放 " + amount + pointName
}

func formatBenefitClearRemark(benefitRow map[string]any, amount int) string {
	identityName := strings.TrimSpace(util.ToString(benefitRow["identity_name"]))
	levelName := strings.TrimSpace(util.ToString(benefitRow["level_name"]))
	return "周期权益清空：" + formatBenefitIdentityLevel(identityName, levelName) + "，清空剩余 " + util.ToString(amount) + benefitPointName(benefitRow)
}

func formatBenefitIdentityLevel(identityName string, levelName string) string {
	parts := make([]string, 0, 2)
	if identityName != "" {
		parts = append(parts, identityName)
	}
	if levelName != "" {
		parts = append(parts, levelName)
	}
	if len(parts) == 0 {
		return "身份权益"
	}
	return strings.Join(parts, "-")
}

func benefitPointName(benefitRow map[string]any) string {
	pointName := strings.TrimSpace(util.ToString(benefitRow["point_name"]))
	if pointName == "" {
		pointName = "积分"
	}
	return pointName
}
