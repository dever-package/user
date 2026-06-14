package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	frontaction "my/package/front/service/action"
	usermodel "my/package/user/model"
)

type PointService struct{}
type UserHook struct{}

const (
	pointChangeIncrease = "increase"
	pointChangeConsume  = "consume"
	pointSourceAdmin    = "admin"
	pointSourceCron     = "cron"

	benefitGrantStatusActive  = 1
	benefitGrantStatusCleared = 2
)

var errUserPointInitRace = errors.New("user point initialization conflict")

type pointAdjustRequest struct {
	userID               uint64
	pointConfigID        uint64
	changeType           string
	source               string
	amount               int
	remark               string
	createdAt            time.Time
	skipGrantConsumption bool
	afterUpdate          func(context.Context, pointAdjustState) error
}

type pointAdjustState struct {
	userPointID   uint64
	userID        uint64
	pointConfigID uint64
	userRow       map[string]any
	pointRow      map[string]any
	userPointRow  map[string]any
	balanceBefore int
	balanceAfter  int
	amount        int
	createdAt     time.Time
}

func (PointService) ProviderAdjust(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	userID := util.ToUint64(payload["user_id"])
	if userID == 0 {
		userID = util.ToUint64(payload["id"])
	}
	if userID == 0 {
		panic(frontaction.NewFieldError("form.user_id", "用户不能为空。"))
	}

	pointConfigID := util.ToUint64(payload["point_config_id"])
	if pointConfigID == 0 {
		pointConfigID = defaultPointConfigID(c.Context())
	}

	changeType := normalizePointChangeType(payload["change_type"])
	if changeType == "" {
		panic(frontaction.NewFieldError("form.change_type", "请选择积分变动类型。"))
	}

	amount := util.ToIntDefault(payload["amount"], 0)
	if amount <= 0 {
		panic(frontaction.NewFieldError("form.amount", "变动积分必须大于 0。"))
	}

	remark := strings.TrimSpace(util.ToString(payload["remark"]))
	if remark == "" {
		panic(frontaction.NewFieldError("form.remark", "请填写积分变动原因。"))
	}

	userPointID, err := adjustUserPoints(c.Context(), pointAdjustRequest{
		userID:        userID,
		pointConfigID: pointConfigID,
		changeType:    changeType,
		source:        pointSourceAdmin,
		amount:        amount,
		remark:        remark,
		createdAt:     time.Now(),
	})
	if err != nil {
		panic(err)
	}

	return map[string]any{
		"id":              userPointID,
		"points_adjusted": true,
	}
}

func (PointService) ProviderAttachUserPointSummary(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	rows := normalizePointRows(payload["rows"])
	if len(rows) == 0 {
		return rows
	}

	userIDs := collectPointUserIDs(rows)
	if len(userIDs) == 0 {
		return rows
	}

	pointRows := usermodel.NewUserPointModel().SelectMap(c.Context(), map[string]any{
		"user_id": userIDs,
	}, map[string]any{
		"order": "user_id asc,point_config_id asc,id asc",
	})
	groupedPoints := groupUserPointRows(pointRows)
	identityRows := usermodel.NewUserIdentityModel().SelectMap(c.Context(), map[string]any{
		"user_id": userIDs,
		"status":  identityStatusEnabled,
	}, map[string]any{
		"order": "user_id asc,identity_id asc,id asc",
	})
	groupedIdentities := groupUserIdentityRows(identityRows)
	for _, row := range rows {
		userID := util.ToUint64(row["id"])
		row["point_summary"] = formatUserPointSummary(groupedPoints[userID])
		row["identity_summary"] = formatUserIdentitySummary(groupedIdentities[userID])
	}
	return rows
}

func (PointService) ProviderAttachUserInfo(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	rows := normalizePointRows(payload["rows"])
	if len(rows) == 0 {
		return rows
	}

	userIDs := collectIDsByField(rows, "user_id")
	if len(userIDs) == 0 {
		return rows
	}
	userRows := usermodel.NewUserModel().SelectMap(c.Context(), map[string]any{
		"id": userIDs,
	}, map[string]any{
		"field": "main.id, main.name, main.mobile",
	})
	users := map[uint64]map[string]any{}
	for _, userRow := range userRows {
		userID := util.ToUint64(userRow["id"])
		if userID == 0 {
			continue
		}
		users[userID] = userRow
	}
	for _, row := range rows {
		userID := util.ToUint64(row["user_id"])
		row["user_info"] = formatUserInfo(row, users[userID])
	}
	return rows
}

func (PointService) ProviderAttachUserIdentityListInfo(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	rows := normalizePointRows(payload["rows"])
	if len(rows) == 0 {
		return rows
	}

	users := map[uint64]map[string]any{}
	userIDs := collectIDsByField(rows, "user_id")
	if len(userIDs) > 0 {
		users = rowsByID(usermodel.NewUserModel().SelectMap(c.Context(), map[string]any{
			"id": userIDs,
		}, map[string]any{
			"field": "main.id, main.name, main.mobile",
		}))
	}

	identities := map[uint64]map[string]any{}
	identityIDs := collectIDsByField(rows, "identity_id")
	if len(identityIDs) > 0 {
		identities = rowsByID(usermodel.NewIdentityModel().SelectMap(c.Context(), map[string]any{
			"id": identityIDs,
		}, map[string]any{
			"field": "main.id, main.name",
		}))
	}

	levels := map[uint64]map[string]any{}
	levelIDs := collectIDsByField(rows, "level_id")
	if len(levelIDs) > 0 {
		levels = rowsByID(usermodel.NewIdentityLevelModel().SelectMap(c.Context(), map[string]any{
			"id": levelIDs,
		}, map[string]any{
			"field": "main.id, main.name, main.level",
		}))
	}

	for _, row := range rows {
		userID := util.ToUint64(row["user_id"])
		identityID := util.ToUint64(row["identity_id"])
		levelID := util.ToUint64(row["level_id"])
		row["user_info"] = formatUserInfo(row, users[userID])
		row["identity_name"] = formatUserIdentityName(row, identities[identityID])
		row["level_name"] = formatUserIdentityLevelName(row, levels[levelID])
	}
	return rows
}

func (UserHook) ProviderBeforeSaveUser(_ *server.Context, params []any) any {
	payload := clonePointPayload(params)
	if nameValue, ok := payload["name"]; ok {
		payload["name"] = strings.TrimSpace(util.ToString(nameValue))
		if payload["name"] == "" {
			panic(frontaction.NewFieldError("form.name", "姓名不能为空。"))
		}
	}
	if mobileValue, ok := payload["mobile"]; ok {
		payload["mobile"] = strings.TrimSpace(util.ToString(mobileValue))
		if payload["mobile"] == "" {
			panic(frontaction.NewFieldError("form.mobile", "手机号不能为空。"))
		}
	}
	if remarkValue, ok := payload["remark"]; ok {
		payload["remark"] = strings.TrimSpace(util.ToString(remarkValue))
	}

	return payload
}

func (UserHook) ProviderAfterSaveUser(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	userID := util.ToUint64(payload["id"])
	if userID == 0 {
		return payload
	}
	if err := syncUserPointSnapshots(c.Context(), userID); err != nil {
		panic(err)
	}
	return payload
}

func (UserHook) ProviderBeforeDeleteUser(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	userID := util.ToUint64(payload["id"])
	if userID == 0 {
		panic("用户不存在")
	}

	logModel := usermodel.NewPointLogModel()
	if logModel.Count(c.Context(), map[string]any{"user_id": userID}) > 0 {
		panic("当前用户已有积分日志，请禁用用户，不要删除。")
	}
	if usermodel.NewUserIdentityLogModel().Count(c.Context(), map[string]any{"user_id": userID}) > 0 {
		panic("当前用户已有身份日志，请禁用用户，不要删除。")
	}
	if usermodel.NewUserBenefitGrantModel().Count(c.Context(), map[string]any{"user_id": userID}) > 0 {
		panic("当前用户已有权益发放记录，请禁用用户，不要删除。")
	}
	usermodel.NewUserPointModel().Delete(c.Context(), map[string]any{"user_id": userID})
	usermodel.NewUserIdentityModel().Delete(c.Context(), map[string]any{"user_id": userID})
	return map[string]any{"id": userID}
}

func adjustUserPoints(ctx context.Context, request pointAdjustRequest) (uint64, error) {
	request = normalizePointAdjustRequest(request)
	for attempt := 0; attempt < 2; attempt++ {
		userPointID, err := adjustUserPointsOnce(ctx, request)
		if errors.Is(err, errUserPointInitRace) {
			continue
		}
		return userPointID, err
	}
	return 0, frontaction.NewFieldError("form.point_config_id", "用户积分初始化冲突，请重试。")
}

func normalizePointAdjustRequest(request pointAdjustRequest) pointAdjustRequest {
	request.changeType = normalizePointChangeType(request.changeType)
	request.source = strings.TrimSpace(request.source)
	if request.source == "" {
		request.source = pointSourceAdmin
	}
	request.remark = strings.TrimSpace(request.remark)
	if request.createdAt.IsZero() {
		request.createdAt = time.Now()
	}
	return request
}

func adjustUserPointsOnce(ctx context.Context, request pointAdjustRequest) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	userModel := usermodel.NewUserModel()
	pointConfigModel := usermodel.NewPointConfigModel()
	userPointModel := usermodel.NewUserPointModel()
	logModel := usermodel.NewPointLogModel()
	var userPointID uint64

	err := orm.Transaction(ctx, func(txCtx context.Context) error {
		userRow := userModel.FindMap(txCtx, map[string]any{"id": request.userID})
		if len(userRow) == 0 {
			return frontaction.NewFieldError("form.user_id", "用户不存在。")
		}
		pointRow := pointConfigModel.FindMap(txCtx, map[string]any{"id": request.pointConfigID})
		if len(pointRow) == 0 {
			return frontaction.NewFieldError("form.point_config_id", "积分配置不存在。")
		}

		userPointRow, err := ensureUserPointRow(txCtx, userPointModel, userRow, pointRow)
		if err != nil {
			return err
		}
		userPointID = util.ToUint64(userPointRow["id"])

		balanceBefore := util.ToIntDefault(userPointRow["balance"], 0)
		totalAdded := util.ToIntDefault(userPointRow["total_added"], 0)
		totalUsed := util.ToIntDefault(userPointRow["total_used"], 0)
		version := util.ToIntDefault(userPointRow["version"], 0)

		balanceAfter := balanceBefore
		updates := userPointSnapshot(userRow, pointRow)
		switch request.changeType {
		case pointChangeIncrease:
			balanceAfter = balanceBefore + request.amount
			updates["balance"] = balanceAfter
			updates["total_added"] = totalAdded + request.amount
		case pointChangeConsume:
			if balanceBefore < request.amount {
				return frontaction.NewFieldError("form.amount", "消耗积分不能超过当前余额。")
			}
			balanceAfter = balanceBefore - request.amount
			updates["balance"] = balanceAfter
			updates["total_used"] = totalUsed + request.amount
		default:
			return frontaction.NewFieldError("form.change_type", "积分变动类型不正确。")
		}

		if err := updateUserPointTotals(txCtx, userPointModel, userPointID, version, updates); err != nil {
			return err
		}

		state := pointAdjustState{
			userPointID:   userPointID,
			userID:        request.userID,
			pointConfigID: request.pointConfigID,
			userRow:       userRow,
			pointRow:      pointRow,
			userPointRow:  userPointRow,
			balanceBefore: balanceBefore,
			balanceAfter:  balanceAfter,
			amount:        request.amount,
			createdAt:     request.createdAt,
		}
		if request.changeType == pointChangeConsume && !request.skipGrantConsumption {
			if err := consumeUserBenefitGrantRemaining(txCtx, request.userID, request.pointConfigID, request.amount, request.createdAt); err != nil {
				return err
			}
		}
		if request.afterUpdate != nil {
			if err := request.afterUpdate(txCtx, state); err != nil {
				return err
			}
		}

		pointSnapshot := pointConfigSnapshot(pointRow)
		logModel.Insert(txCtx, map[string]any{
			"user_point_id":         userPointID,
			"user_id":               request.userID,
			"user_name":             strings.TrimSpace(util.ToString(userRow["name"])),
			"user_mobile":           strings.TrimSpace(util.ToString(userRow["mobile"])),
			"point_config_id":       request.pointConfigID,
			"point_name":            pointSnapshot.name,
			"point_symbol":          pointSnapshot.symbol,
			"point_symbol_position": pointSnapshot.symbolPosition,
			"change_type":           request.changeType,
			"source":                request.source,
			"amount":                request.amount,
			"balance_before":        balanceBefore,
			"balance_after":         balanceAfter,
			"remark":                request.remark,
			"created_at":            request.createdAt,
		})
		return nil
	})
	return userPointID, err
}

func consumeUserBenefitGrantRemaining(ctx context.Context, userID uint64, pointConfigID uint64, amount int, now time.Time) error {
	if amount <= 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	remainingToConsume := amount
	grantModel := usermodel.NewUserBenefitGrantModel()
	rows := grantModel.SelectMap(ctx, map[string]any{
		"user_id":         userID,
		"point_config_id": pointConfigID,
		"status":          benefitGrantStatusActive,
	}, map[string]any{
		"order": "cycle_start_at asc,created_at asc,id asc",
	})
	for _, row := range rows {
		grantID := util.ToUint64(row["id"])
		grantRemaining := util.ToIntDefault(row["remaining_amount"], 0)
		if grantID == 0 || grantRemaining <= 0 {
			continue
		}
		deductAmount := grantRemaining
		if deductAmount > remainingToConsume {
			deductAmount = remainingToConsume
		}
		nextRemaining := grantRemaining - deductAmount
		updates := map[string]any{"remaining_amount": nextRemaining}
		if nextRemaining <= 0 {
			updates["remaining_amount"] = 0
			updates["status"] = benefitGrantStatusCleared
			updates["cleared_at"] = now
		}
		grantModel.Update(ctx, map[string]any{"id": grantID}, updates, false)
		remainingToConsume -= deductAmount
		if remainingToConsume <= 0 {
			return nil
		}
	}
	return nil
}

func updateUserPointTotals(ctx context.Context, userPointModel *orm.Model[usermodel.UserPoint], userPointID uint64, version int, updates map[string]any) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok && errors.Is(recoveredErr, orm.ErrVersionConflict) {
				err = frontaction.NewFieldError("form.amount", "积分已发生变化，请刷新后重试。")
				return
			}
			panic(recovered)
		}
	}()

	updated := userPointModel.Update(ctx, map[string]any{
		"id":      userPointID,
		"version": version,
	}, updates, true)
	if updated == 0 {
		return frontaction.NewFieldError("form.amount", "积分已发生变化，请刷新后重试。")
	}
	return nil
}

func syncUserPointSnapshots(ctx context.Context, userID uint64) error {
	userModel := usermodel.NewUserModel()
	pointConfigModel := usermodel.NewPointConfigModel()
	userPointModel := usermodel.NewUserPointModel()
	userRow := userModel.FindMap(ctx, map[string]any{"id": userID})
	if len(userRow) == 0 {
		return nil
	}
	defaultPointRow := pointConfigModel.FindMap(ctx, map[string]any{"id": uint64(1)})
	if len(defaultPointRow) == 0 {
		defaultPointRow = firstPointConfigRow(ctx, pointConfigModel)
	}
	if err := ensureDefaultUserPointRow(ctx, userPointModel, userRow, defaultPointRow); err != nil {
		return err
	}
	userPointModel.Update(ctx, map[string]any{"user_id": userID}, map[string]any{
		"user_name":   strings.TrimSpace(util.ToString(userRow["name"])),
		"user_mobile": strings.TrimSpace(util.ToString(userRow["mobile"])),
	}, false)
	usermodel.NewUserIdentityModel().Update(ctx, map[string]any{"user_id": userID}, map[string]any{
		"user_name":   strings.TrimSpace(util.ToString(userRow["name"])),
		"user_mobile": strings.TrimSpace(util.ToString(userRow["mobile"])),
	}, false)
	return nil
}

func ensureDefaultUserPointRow(ctx context.Context, userPointModel *orm.Model[usermodel.UserPoint], userRow map[string]any, defaultPointRow map[string]any) error {
	if len(defaultPointRow) == 0 {
		return nil
	}
	if _, err := ensureUserPointRow(ctx, userPointModel, userRow, defaultPointRow); err != nil {
		if !errors.Is(err, errUserPointInitRace) {
			return err
		}
		if len(userPointModel.FindMap(ctx, map[string]any{
			"user_id":         util.ToUint64(userRow["id"]),
			"point_config_id": util.ToUint64(defaultPointRow["id"]),
		})) == 0 {
			return err
		}
	}
	return nil
}

func defaultPointConfigID(ctx context.Context) uint64 {
	pointConfigModel := usermodel.NewPointConfigModel()
	defaultPointRow := pointConfigModel.FindMap(ctx, map[string]any{"id": uint64(1)})
	if len(defaultPointRow) == 0 {
		defaultPointRow = firstPointConfigRow(ctx, pointConfigModel)
	}
	return util.ToUint64(defaultPointRow["id"])
}

func firstPointConfigRow(ctx context.Context, pointConfigModel *orm.Model[usermodel.PointConfig]) map[string]any {
	rows := pointConfigModel.SelectMap(ctx, nil, map[string]any{
		"order": "id asc",
		"limit": 1,
	})
	if len(rows) == 0 {
		return map[string]any{}
	}
	return rows[0]
}

func ensureUserPointRow(ctx context.Context, userPointModel *orm.Model[usermodel.UserPoint], userRow map[string]any, pointRow map[string]any) (map[string]any, error) {
	userID := util.ToUint64(userRow["id"])
	pointConfigID := util.ToUint64(pointRow["id"])
	filter := map[string]any{
		"user_id":         userID,
		"point_config_id": pointConfigID,
	}
	userPointRow := userPointModel.FindMap(ctx, filter)
	if len(userPointRow) > 0 {
		return userPointRow, nil
	}

	id, err := insertUserPointRow(ctx, userPointModel, userPointSnapshot(userRow, pointRow))
	if err != nil {
		if isUniqueConflictError(err) {
			return nil, errUserPointInitRace
		}
		return nil, err
	}
	if id > 0 {
		userPointRow = userPointModel.FindMap(ctx, map[string]any{"id": id})
	}
	if len(userPointRow) == 0 {
		userPointRow = userPointModel.FindMap(ctx, filter)
	}
	if len(userPointRow) == 0 {
		return nil, frontaction.NewFieldError("form.point_config_id", "用户积分初始化失败。")
	}
	return userPointRow, nil
}

func insertUserPointRow(ctx context.Context, userPointModel *orm.Model[usermodel.UserPoint], record map[string]any) (id uint64, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok {
				err = recoveredErr
				return
			}
			err = fmt.Errorf("%v", recovered)
		}
	}()
	id = util.ToUint64(userPointModel.Insert(ctx, record))
	return id, nil
}

func isUniqueConflictError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "duplicate key value violates unique constraint") ||
		strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "duplicate entry")
}

func userPointSnapshot(userRow map[string]any, pointRow map[string]any) map[string]any {
	pointSnapshot := pointConfigSnapshot(pointRow)
	return map[string]any{
		"user_id":               util.ToUint64(userRow["id"]),
		"user_name":             strings.TrimSpace(util.ToString(userRow["name"])),
		"user_mobile":           strings.TrimSpace(util.ToString(userRow["mobile"])),
		"point_config_id":       util.ToUint64(pointRow["id"]),
		"point_name":            pointSnapshot.name,
		"point_symbol":          pointSnapshot.symbol,
		"point_symbol_position": pointSnapshot.symbolPosition,
	}
}

type pointConfigSnapshotValue struct {
	name           string
	symbol         string
	symbolPosition int16
}

func pointConfigSnapshot(pointRow map[string]any) pointConfigSnapshotValue {
	name := strings.TrimSpace(util.ToString(pointRow["name"]))
	symbol := strings.TrimSpace(util.ToString(pointRow["symbol"]))
	if symbol == "" {
		symbol = name
	}
	position := util.ToIntDefault(pointRow["symbol_position"], 2)
	if position != 1 {
		position = 2
	}
	return pointConfigSnapshotValue{
		name:           name,
		symbol:         symbol,
		symbolPosition: int16(position),
	}
}

func normalizePointRows(value any) []map[string]any {
	if rows, ok := value.([]map[string]any); ok {
		return rows
	}
	rawRows, ok := value.([]any)
	if !ok {
		return nil
	}
	rows := make([]map[string]any, 0, len(rawRows))
	for _, rawRow := range rawRows {
		row, ok := rawRow.(map[string]any)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func collectPointUserIDs(rows []map[string]any) []uint64 {
	seen := map[uint64]struct{}{}
	userIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		userID := util.ToUint64(row["id"])
		if userID == 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func collectIDsByField(rows []map[string]any, field string) []uint64 {
	seen := map[uint64]struct{}{}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		id := util.ToUint64(row[field])
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func rowsByID(rows []map[string]any) map[uint64]map[string]any {
	grouped := map[uint64]map[string]any{}
	for _, row := range rows {
		id := util.ToUint64(row["id"])
		if id == 0 {
			continue
		}
		grouped[id] = row
	}
	return grouped
}

func groupUserPointRows(rows []map[string]any) map[uint64][]map[string]any {
	grouped := map[uint64][]map[string]any{}
	for _, row := range rows {
		userID := util.ToUint64(row["user_id"])
		if userID == 0 {
			continue
		}
		grouped[userID] = append(grouped[userID], row)
	}
	return grouped
}

func groupUserIdentityRows(rows []map[string]any) map[uint64][]map[string]any {
	grouped := map[uint64][]map[string]any{}
	for _, row := range rows {
		userID := util.ToUint64(row["user_id"])
		if userID == 0 {
			continue
		}
		grouped[userID] = append(grouped[userID], row)
	}
	return grouped
}

func formatUserPointSummary(rows []map[string]any) string {
	if len(rows) == 0 {
		return "暂无积分"
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, formatUserPointLine(row))
	}
	return strings.Join(lines, "；")
}

func formatUserPointLine(row map[string]any) string {
	name := strings.TrimSpace(util.ToString(row["point_name"]))
	symbol := strings.TrimSpace(util.ToString(row["point_symbol"]))
	if name == "" {
		name = "积分"
	}
	if symbol == "" {
		symbol = name
	}
	balance := util.ToIntDefault(row["balance"], 0)
	if util.ToIntDefault(row["point_symbol_position"], 2) == 1 {
		return fmt.Sprintf("%s：%s%d", name, symbol, balance)
	}
	return fmt.Sprintf("%s：%d%s", name, balance, symbol)
}

func formatUserIdentitySummary(rows []map[string]any) string {
	if len(rows) == 0 {
		return "暂无身份"
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, formatUserIdentityLine(row))
	}
	return strings.Join(lines, "；")
}

func formatUserIdentityLine(row map[string]any) string {
	identityName := strings.TrimSpace(util.ToString(row["identity_name"]))
	levelName := strings.TrimSpace(util.ToString(row["level_name"]))
	if identityName == "" {
		identityName = "身份"
	}
	if levelName == "" {
		level := util.ToIntDefault(row["level"], 0)
		if level > 0 {
			levelName = fmt.Sprintf("%d级", level)
		} else {
			levelName = "未设置等级"
		}
	}
	return fmt.Sprintf("%s：%s", identityName, levelName)
}

func formatUserInfo(row map[string]any, userRow map[string]any) string {
	name := strings.TrimSpace(util.ToString(userRow["name"]))
	mobile := strings.TrimSpace(util.ToString(userRow["mobile"]))
	if name == "" {
		name = strings.TrimSpace(util.ToString(row["user_name"]))
	}
	if mobile == "" {
		mobile = strings.TrimSpace(util.ToString(row["user_mobile"]))
	}
	values := []string{
		name,
		mobile,
		strings.TrimSpace(util.ToString(row["user_id"])),
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && value != "0" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "-")
}

func formatUserIdentityName(row map[string]any, identityRow map[string]any) string {
	name := strings.TrimSpace(util.ToString(identityRow["name"]))
	if name == "" {
		name = strings.TrimSpace(util.ToString(row["identity_name"]))
	}
	if name == "" {
		return "-"
	}
	return name
}

func formatUserIdentityLevelName(row map[string]any, levelRow map[string]any) string {
	name := strings.TrimSpace(util.ToString(levelRow["name"]))
	if name == "" {
		name = strings.TrimSpace(util.ToString(row["level_name"]))
	}
	if name == "" {
		return "-"
	}
	return name
}

func clonePointPayload(params []any) map[string]any {
	if len(params) == 0 {
		return map[string]any{}
	}
	record, _ := params[0].(map[string]any)
	if record == nil {
		return map[string]any{}
	}
	return util.CloneMap(record)
}

func normalizePointChangeType(value any) string {
	switch strings.TrimSpace(util.ToString(value)) {
	case pointChangeIncrease:
		return pointChangeIncrease
	case pointChangeConsume:
		return pointChangeConsume
	default:
		return ""
	}
}
