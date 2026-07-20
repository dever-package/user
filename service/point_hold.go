package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/util"

	frontaction "github.com/dever-package/front/service/action"
	usermodel "github.com/dever-package/user/model"
)

const (
	defaultPointHoldDuration = 24 * time.Hour
	pointHoldBusinessAbility = "ability"
	pointSourceBilling       = "billing"
)

type PointReserveRequest struct {
	BusinessKey   string
	BusinessType  string
	UserID        uint64
	PointConfigID uint64
	Amount        int
	Remark        string
	ExpiresAt     time.Time
}

type PointSettleRequest struct {
	BusinessKey  string
	Amount       int
	Remark       string
	AllowPartial bool
}

type PointReleaseRequest struct {
	BusinessKey string
	Remark      string
}

type PointHoldResult struct {
	ID             uint64
	BusinessKey    string
	UserPointID    uint64
	UserID         uint64
	PointConfigID  uint64
	ReservedAmount int
	SettledAmount  int
	Status         int16
	ExpiresAt      time.Time
}

func ReservePoints(ctx context.Context, request PointReserveRequest) (PointHoldResult, error) {
	request.BusinessKey = normalizePointHoldBusinessKey(request.BusinessKey)
	if err := validatePointHoldBusinessKey(request.BusinessKey); err != nil {
		return PointHoldResult{}, err
	}
	request.BusinessType = strings.TrimSpace(request.BusinessType)
	if request.BusinessType == "" {
		request.BusinessType = pointHoldBusinessAbility
	}
	request.Remark = strings.TrimSpace(request.Remark)
	if request.UserID == 0 {
		return PointHoldResult{}, frontaction.NewFieldError("user_id", "积分预占用户不能为空。")
	}
	if request.PointConfigID == 0 {
		return PointHoldResult{}, frontaction.NewFieldError("point_config_id", "积分预占类型不能为空。")
	}
	if err := validatePointChangeAmount(request.Amount, "amount"); err != nil {
		return PointHoldResult{}, err
	}
	if existing := pointHoldByBusinessKey(ctx, request.BusinessKey); len(existing) > 0 {
		return validateReservedPointHold(ctx, existing, request)
	}

	now := time.Now()
	if request.ExpiresAt.IsZero() || !request.ExpiresAt.After(now) {
		request.ExpiresAt = now.Add(defaultPointHoldDuration)
	}
	var holdID uint64
	var err error
	for attempt := 0; attempt < pointAdjustmentMaxAttempts; attempt++ {
		holdID, err = reservePointsOnce(ctx, request, now)
		if errors.Is(err, errUserPointVersionRace) {
			if waitErr := waitPointAdjustmentRetry(ctx, attempt); waitErr != nil {
				return PointHoldResult{}, waitErr
			}
			continue
		}
		break
	}
	if errors.Is(err, errUserPointVersionRace) {
		return PointHoldResult{}, frontaction.NewFieldError("amount", "积分已发生变化，请重试。")
	}
	if err != nil {
		if isUniqueConflictError(err) {
			if existing := pointHoldByBusinessKey(ctx, request.BusinessKey); len(existing) > 0 {
				return validateReservedPointHold(ctx, existing, request)
			}
		}
		return PointHoldResult{}, err
	}
	row := usermodel.NewPointHoldModel().FindMap(ctx, map[string]any{"id": holdID})
	return validateReservedPointHold(ctx, row, request)
}

func reservePointsOnce(ctx context.Context, request PointReserveRequest, now time.Time) (uint64, error) {
	var holdID uint64
	err := orm.Transaction(ctx, func(txCtx context.Context) error {
		if existing := pointHoldByBusinessKey(txCtx, request.BusinessKey); len(existing) > 0 {
			holdID = util.ToUint64(existing["id"])
			return nil
		}

		userRow := usermodel.NewUserModel().FindMap(txCtx, map[string]any{"id": request.UserID})
		if len(userRow) == 0 {
			return frontaction.NewFieldError("user_id", "积分预占用户不存在。")
		}
		pointRow := usermodel.NewPointConfigModel().FindMap(txCtx, map[string]any{"id": request.PointConfigID})
		if len(pointRow) == 0 {
			return frontaction.NewFieldError("point_config_id", "积分预占类型不存在。")
		}

		userPointModel := usermodel.NewUserPointModel()
		userPointRow, err := ensureUserPointRow(txCtx, userPointModel, userRow, pointRow)
		if err != nil {
			return err
		}
		userPointID := util.ToUint64(userPointRow["id"])
		balance := util.ToIntDefault(userPointRow["balance"], 0)
		available := balance - activePointHoldAmount(txCtx, userPointID, 0, now)
		if available < request.Amount {
			return frontaction.NewFieldError("amount", "当前可用积分不足。")
		}

		updates := userPointSnapshot(userRow, pointRow)
		updates["balance"] = balance
		if err := updateUserPointTotals(txCtx, userPointModel, userPointID, util.ToIntDefault(userPointRow["version"], 0), updates); err != nil {
			return err
		}

		holdID, err = insertPointHoldRow(txCtx, pointHoldRecord(request, userPointID, userRow, pointRow, now))
		return err
	})
	return holdID, err
}

func SettlePoints(ctx context.Context, request PointSettleRequest) (PointHoldResult, error) {
	request.BusinessKey = normalizePointHoldBusinessKey(request.BusinessKey)
	if err := validatePointHoldBusinessKey(request.BusinessKey); err != nil {
		return PointHoldResult{}, err
	}
	request.Remark = strings.TrimSpace(request.Remark)
	if request.Amount < 0 {
		return PointHoldResult{}, frontaction.NewFieldError("amount", "结算积分不能小于 0。")
	}
	if request.Amount > maxPointChangeAmount {
		return PointHoldResult{}, frontaction.NewFieldError("amount", fmt.Sprintf("单次结算积分不能超过 %d。", maxPointChangeAmount))
	}

	row := pointHoldByBusinessKey(ctx, request.BusinessKey)
	if len(row) == 0 {
		return PointHoldResult{}, fmt.Errorf("积分预占不存在")
	}
	row = expirePointHoldIfNeeded(ctx, row, time.Now())
	status := int16(util.ToIntDefault(row["status"], 0))
	if status == usermodel.PointHoldStatusSettled {
		return pointHoldResult(row), nil
	}
	if status != usermodel.PointHoldStatusActive {
		return pointHoldResult(row), fmt.Errorf("积分预占已失效，不能结算")
	}
	reservedAmount := util.ToIntDefault(row["reserved_amount"], 0)
	if request.Amount > reservedAmount {
		if !request.AllowPartial {
			return pointHoldResult(row), frontaction.NewFieldError("amount", "结算积分不能超过预占积分。")
		}
		request.Amount = reservedAmount
	}
	if request.Amount == 0 {
		if err := settleEmptyPointHold(ctx, row, request.Remark); err != nil {
			return PointHoldResult{}, err
		}
		return pointHoldResult(pointHoldByBusinessKey(ctx, request.BusinessKey)), nil
	}

	holdID := util.ToUint64(row["id"])
	remark := request.Remark
	if remark == "" {
		remark = strings.TrimSpace(util.ToString(row["remark"]))
	}
	state, err := adjustUserPointsState(ctx, pointAdjustRequest{
		userID:        util.ToUint64(row["user_id"]),
		pointConfigID: util.ToUint64(row["point_config_id"]),
		pointHoldID:   holdID,
		businessKey:   request.BusinessKey,
		changeType:    pointChangeConsume,
		source:        pointSourceBilling,
		amount:        request.Amount,
		remark:        remark,
		createdAt:     time.Now(),
		allowPartial:  request.AllowPartial,
		afterUpdate: func(txCtx context.Context, adjusted pointAdjustState) error {
			now := time.Now()
			updated := usermodel.NewPointHoldModel().Update(txCtx, map[string]any{
				"id":     holdID,
				"status": usermodel.PointHoldStatusActive,
			}, map[string]any{
				"settled_amount": adjusted.amount,
				"status":         usermodel.PointHoldStatusSettled,
				"remark":         remark,
				"settled_at":     now,
			}, false)
			if updated == 0 {
				return fmt.Errorf("积分预占状态已变化，结算已取消")
			}
			return nil
		},
	})
	if err != nil {
		return PointHoldResult{}, err
	}
	result := pointHoldResult(pointHoldByBusinessKey(ctx, request.BusinessKey))
	result.SettledAmount = state.amount
	return result, nil
}

func ReleasePoints(ctx context.Context, request PointReleaseRequest) (PointHoldResult, error) {
	request.BusinessKey = normalizePointHoldBusinessKey(request.BusinessKey)
	if err := validatePointHoldBusinessKey(request.BusinessKey); err != nil {
		return PointHoldResult{}, err
	}
	request.Remark = strings.TrimSpace(request.Remark)
	row := pointHoldByBusinessKey(ctx, request.BusinessKey)
	if len(row) == 0 {
		return PointHoldResult{}, fmt.Errorf("积分预占不存在")
	}
	row = expirePointHoldIfNeeded(ctx, row, time.Now())
	if int16(util.ToIntDefault(row["status"], 0)) != usermodel.PointHoldStatusActive {
		return pointHoldResult(row), nil
	}
	now := time.Now()
	updates := map[string]any{
		"status":      usermodel.PointHoldStatusReleased,
		"released_at": now,
	}
	if request.Remark != "" {
		updates["remark"] = request.Remark
	}
	usermodel.NewPointHoldModel().Update(ctx, map[string]any{
		"id":     util.ToUint64(row["id"]),
		"status": usermodel.PointHoldStatusActive,
	}, updates, false)
	return pointHoldResult(pointHoldByBusinessKey(ctx, request.BusinessKey)), nil
}

func FindPointHold(ctx context.Context, businessKey string) (PointHoldResult, bool) {
	businessKey = normalizePointHoldBusinessKey(businessKey)
	if validatePointHoldBusinessKey(businessKey) != nil {
		return PointHoldResult{}, false
	}
	row := pointHoldByBusinessKey(ctx, businessKey)
	if len(row) == 0 {
		return PointHoldResult{}, false
	}
	row = expirePointHoldIfNeeded(ctx, row, time.Now())
	return pointHoldResult(row), true
}

func activePointHoldAmount(ctx context.Context, userPointID uint64, excludeHoldID uint64, now time.Time) int {
	if userPointID == 0 {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	model := usermodel.NewPointHoldModel()
	model.Update(ctx, map[string]any{
		"user_point_id": userPointID,
		"status":        usermodel.PointHoldStatusActive,
		"expires_at":    map[string]any{"lte": now},
	}, map[string]any{
		"status":      usermodel.PointHoldStatusExpired,
		"released_at": now,
	}, false)
	filters := map[string]any{
		"user_point_id": userPointID,
		"status":        usermodel.PointHoldStatusActive,
		"expires_at":    map[string]any{"gt": now},
	}
	if excludeHoldID > 0 {
		filters["id"] = map[string]any{"neq": excludeHoldID}
	}
	rows := model.SelectMap(ctx, filters, map[string]any{
		"field": "main.id,main.reserved_amount",
		"order": "id asc",
	})
	total := 0
	for _, row := range rows {
		amount := util.ToIntDefault(row["reserved_amount"], 0)
		if amount <= 0 {
			continue
		}
		if total > maxStoredPointValue-amount {
			return maxStoredPointValue
		}
		total += amount
	}
	return total
}

func ExpirePointHolds(ctx context.Context, now time.Time) int64 {
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}
	return usermodel.NewPointHoldModel().Update(ctx, map[string]any{
		"status":     usermodel.PointHoldStatusActive,
		"expires_at": map[string]any{"lte": now},
	}, map[string]any{
		"status":      usermodel.PointHoldStatusExpired,
		"released_at": now,
	}, false)
}

func validateReservedPointHold(ctx context.Context, row map[string]any, request PointReserveRequest) (PointHoldResult, error) {
	if len(row) == 0 {
		return PointHoldResult{}, fmt.Errorf("积分预占保存失败")
	}
	row = expirePointHoldIfNeeded(ctx, row, time.Now())
	if util.ToUint64(row["user_id"]) != request.UserID ||
		util.ToUint64(row["point_config_id"]) != request.PointConfigID ||
		util.ToIntDefault(row["reserved_amount"], 0) != request.Amount ||
		strings.TrimSpace(util.ToString(row["business_type"])) != request.BusinessType {
		return PointHoldResult{}, fmt.Errorf("业务幂等键已被其他积分预占使用")
	}
	return pointHoldResult(row), nil
}

func settleEmptyPointHold(ctx context.Context, row map[string]any, remark string) error {
	now := time.Now()
	updated := usermodel.NewPointHoldModel().Update(ctx, map[string]any{
		"id":     util.ToUint64(row["id"]),
		"status": usermodel.PointHoldStatusActive,
	}, map[string]any{
		"settled_amount": 0,
		"status":         usermodel.PointHoldStatusSettled,
		"remark":         remark,
		"settled_at":     now,
	}, false)
	if updated == 0 {
		latest := pointHoldByBusinessKey(ctx, util.ToString(row["business_key"]))
		if int16(util.ToIntDefault(latest["status"], 0)) == usermodel.PointHoldStatusSettled {
			return nil
		}
		return fmt.Errorf("积分预占状态已变化，结算已取消")
	}
	return nil
}

func pointHoldRecord(request PointReserveRequest, userPointID uint64, userRow map[string]any, pointRow map[string]any, now time.Time) map[string]any {
	pointSnapshot := pointConfigSnapshot(pointRow)
	return map[string]any{
		"business_key":          request.BusinessKey,
		"business_type":         request.BusinessType,
		"user_point_id":         userPointID,
		"user_id":               request.UserID,
		"user_name":             strings.TrimSpace(util.ToString(userRow["name"])),
		"user_mobile":           strings.TrimSpace(util.ToString(userRow["mobile"])),
		"point_config_id":       request.PointConfigID,
		"point_name":            pointSnapshot.name,
		"point_symbol":          pointSnapshot.symbol,
		"point_symbol_position": pointSnapshot.symbolPosition,
		"reserved_amount":       request.Amount,
		"settled_amount":        0,
		"status":                usermodel.PointHoldStatusActive,
		"remark":                request.Remark,
		"expires_at":            request.ExpiresAt,
		"created_at":            now,
	}
}

func insertPointHoldRow(ctx context.Context, record map[string]any) (id uint64, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok {
				err = recoveredErr
				return
			}
			err = fmt.Errorf("%v", recovered)
		}
	}()
	id = util.ToUint64(usermodel.NewPointHoldModel().Insert(ctx, record))
	if id == 0 {
		return 0, fmt.Errorf("积分预占保存失败")
	}
	return id, nil
}

func pointHoldByBusinessKey(ctx context.Context, businessKey string) map[string]any {
	if strings.TrimSpace(businessKey) == "" {
		return map[string]any{}
	}
	return usermodel.NewPointHoldModel().FindMap(ctx, map[string]any{"business_key": businessKey})
}

func expirePointHoldIfNeeded(ctx context.Context, row map[string]any, now time.Time) map[string]any {
	if len(row) == 0 || int16(util.ToIntDefault(row["status"], 0)) != usermodel.PointHoldStatusActive {
		return row
	}
	expiresAt := normalizeUserIdentityTime(row["expires_at"])
	if expiresAt.IsZero() || expiresAt.After(now) {
		return row
	}
	usermodel.NewPointHoldModel().Update(ctx, map[string]any{
		"id":     util.ToUint64(row["id"]),
		"status": usermodel.PointHoldStatusActive,
	}, map[string]any{
		"status":      usermodel.PointHoldStatusExpired,
		"released_at": now,
	}, false)
	return usermodel.NewPointHoldModel().FindMap(ctx, map[string]any{"id": util.ToUint64(row["id"])})
}

func pointHoldResult(row map[string]any) PointHoldResult {
	return PointHoldResult{
		ID:             util.ToUint64(row["id"]),
		BusinessKey:    strings.TrimSpace(util.ToString(row["business_key"])),
		UserPointID:    util.ToUint64(row["user_point_id"]),
		UserID:         util.ToUint64(row["user_id"]),
		PointConfigID:  util.ToUint64(row["point_config_id"]),
		ReservedAmount: util.ToIntDefault(row["reserved_amount"], 0),
		SettledAmount:  util.ToIntDefault(row["settled_amount"], 0),
		Status:         int16(util.ToIntDefault(row["status"], 0)),
		ExpiresAt:      normalizeUserIdentityTime(row["expires_at"]),
	}
}

func normalizePointHoldBusinessKey(value string) string {
	return strings.TrimSpace(value)
}

func validatePointHoldBusinessKey(value string) error {
	if value == "" {
		return frontaction.NewFieldError("business_key", "积分预占业务键不能为空。")
	}
	if len(value) > 128 {
		return frontaction.NewFieldError("business_key", "积分预占业务键不能超过 128 个字符。")
	}
	return nil
}
