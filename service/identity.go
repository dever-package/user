package service

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	frontaction "github.com/dever-package/front/service/action"
	frontrecord "github.com/dever-package/front/service/record"
	usermodel "github.com/dever-package/user/model"
)

const (
	identityStatusEnabled  = 1
	identityStatusDisabled = 2

	levelDurationReset = 1
	levelDurationRenew = 2

	levelUpgradePay      = 1
	levelUpgradeRegister = 2
	levelUpgradeGift     = 3

	levelPayDiff = 1
	levelPayFull = 2
)

type UserOptionService struct{}

type UserIdentityService struct{}

func (UserOptionService) ProviderLoadIdentities(c *server.Context, _ []any) any {
	identityModel := frontrecord.Resolve("user.NewIdentityModel")
	if identityModel == nil {
		return []map[string]any{}
	}

	rows := identityModel.SelectMap(c.Context(), nil, map[string]any{
		"field": "main.id, main.name, main.status, main.sort",
		"order": "main.sort asc, main.id asc",
	})
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(util.ToString(row["name"]))
		result = append(result, map[string]any{
			"id":     util.ToUint64(row["id"]),
			"value":  name,
			"label":  name,
			"name":   name,
			"status": util.ToIntDefault(row["status"], 0),
			"sort":   util.ToIntDefault(row["sort"], 0),
		})
	}
	return result
}

func (UserOptionService) ProviderLoadUsers(c *server.Context, _ []any) any {
	keyword := strings.TrimSpace(c.Input("keyword"))
	rows := usermodel.NewUserModel().SelectMap(c.Context(), userOptionFilters(keyword), map[string]any{
		"field":    "main.id, main.name, main.mobile",
		"order":    "main.id desc",
		"pageSize": normalizeUserOptionPageSize(c.Input("pageSize")),
	})
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		label := formatUserOptionInfo(row)
		result = append(result, map[string]any{
			"id":     util.ToUint64(row["id"]),
			"value":  label,
			"label":  label,
			"name":   label,
			"mobile": strings.TrimSpace(util.ToString(row["mobile"])),
		})
	}
	return result
}

func (UserOptionService) ProviderLoadPointConfigs(c *server.Context, _ []any) any {
	rows := usermodel.NewPointConfigModel().SelectMap(c.Context(), nil, map[string]any{
		"field": "main.id, main.name, main.exchange_rate, main.symbol, main.symbol_position",
		"order": "main.id asc",
	})
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(util.ToString(row["name"]))
		result = append(result, map[string]any{
			"id":              util.ToUint64(row["id"]),
			"value":           name,
			"label":           name,
			"name":            name,
			"benefit_type":    benefitTypeRewardPoint,
			"exchange_rate":   util.ToIntDefault(row["exchange_rate"], 0),
			"symbol":          strings.TrimSpace(util.ToString(row["symbol"])),
			"symbol_position": util.ToIntDefault(row["symbol_position"], 1),
			"leaf":            true,
		})
	}
	return result
}

func (UserOptionService) ProviderLoadBenefitTypes(_ *server.Context, _ []any) any {
	return []map[string]any{
		{
			"id":    benefitTypeRewardPoint,
			"value": "奖励积分",
			"label": "奖励积分",
			"name":  "奖励积分",
			"leaf":  false,
		},
	}
}

func (UserOptionService) ProviderLoadBenefitItems(c *server.Context, params []any) any {
	if serviceOptionBenefitType(params) != benefitTypeRewardPoint {
		return []map[string]any{}
	}
	return (UserOptionService{}).ProviderLoadPointConfigs(c, nil)
}

func serviceOptionBenefitType(params []any) string {
	if len(params) == 0 {
		return benefitTypeRewardPoint
	}
	payload, ok := params[0].(map[string]any)
	if !ok {
		return benefitTypeRewardPoint
	}
	for _, key := range []string{"benefit_type", "parent_id", "parentId", "id"} {
		if value := strings.TrimSpace(util.ToString(payload[key])); isMeaningfulBenefitTypeValue(value) {
			return value
		}
	}
	if value := strings.TrimSpace(util.ToString(payload["value"])); value == benefitTypeRewardPoint {
		return value
	}
	return benefitTypeRewardPoint
}

func isMeaningfulBenefitTypeValue(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "0":
		return false
	default:
		return true
	}
}

func userOptionFilters(keyword string) any {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil
	}
	return map[string]any{
		"or": []any{
			map[string]any{"main.name": map[string]any{"like": "%" + keyword + "%"}},
			map[string]any{"main.mobile": map[string]any{"like": "%" + keyword + "%"}},
		},
	}
}

func normalizeUserOptionPageSize(value any) int {
	pageSize := util.ToIntDefault(value, 50)
	if pageSize <= 0 {
		return 50
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

func formatUserOptionInfo(row map[string]any) string {
	values := []string{
		strings.TrimSpace(util.ToString(row["name"])),
		strings.TrimSpace(util.ToString(row["mobile"])),
		strings.TrimSpace(util.ToString(row["id"])),
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

func (UserHook) ProviderBeforeSaveIdentity(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	if isUserPartialRecord(payload) {
		normalizePresentIdentitySortAndStatus(payload)
		return payload
	}

	name := strings.TrimSpace(util.ToString(payload["name"]))
	if name == "" {
		panic(frontaction.NewFieldError("form.name", "身份名称不能为空。"))
	}
	payload["name"] = name
	purchasePointID := util.ToUint64(payload["purchase_point_id"])
	if purchasePointID == 0 {
		purchasePointID = defaultPointConfigID(c.Context())
	}
	if len(usermodel.NewPointConfigModel().FindMap(c.Context(), map[string]any{"id": purchasePointID})) == 0 {
		panic(frontaction.NewFieldError("form.purchase_point_id", "购买积分不存在。"))
	}
	payload["purchase_point_id"] = purchasePointID
	normalizePresentIdentitySortAndStatus(payload)
	return payload
}

func (UserHook) ProviderBeforeDeleteIdentity(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	identityID := util.ToUint64(payload["id"])
	if identityID == 0 {
		panic("身份不存在")
	}

	levelModel := usermodel.NewIdentityLevelModel()
	if levelModel.Count(c.Context(), map[string]any{"identity_id": identityID}) > 0 {
		panic("当前身份下仍有等级，请先处理等级后再删除")
	}
	if usermodel.NewUserIdentityModel().Count(c.Context(), map[string]any{"identity_id": identityID}) > 0 {
		panic("当前身份已分配给用户，请先处理用户身份。")
	}
	if usermodel.NewUserBenefitGrantModel().Count(c.Context(), map[string]any{"identity_id": identityID}) > 0 {
		panic("当前身份已有权益发放记录，不能删除。")
	}
	return map[string]any{"id": identityID}
}

func (UserHook) ProviderBeforeSaveIdentityLevel(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	if isUserPartialRecord(payload) {
		normalizePresentIdentitySortAndStatus(payload)
		return payload
	}

	identityID := util.ToUint64(payload["identity_id"])
	if identityID == 0 {
		panic(frontaction.NewFieldError("form.identity_id", "所属身份不能为空。"))
	}
	if len(usermodel.NewIdentityModel().FindMap(c.Context(), map[string]any{"id": identityID})) == 0 {
		panic(frontaction.NewFieldError("form.identity_id", "所属身份不存在。"))
	}
	levelID := util.ToUint64(payload["id"])
	if levelID > 0 {
		currentLevel := usermodel.NewIdentityLevelModel().FindMap(c.Context(), map[string]any{"id": levelID})
		if len(currentLevel) > 0 &&
			util.ToUint64(currentLevel["identity_id"]) != identityID &&
			usermodel.NewUserIdentityModel().Count(c.Context(), map[string]any{"level_id": levelID}) > 0 {
			panic(frontaction.NewFieldError("form.identity_id", "当前等级已分配给用户，不能切换所属身份。"))
		}
	}
	payload["identity_id"] = identityID

	name := strings.TrimSpace(util.ToString(payload["name"]))
	if name == "" {
		panic(frontaction.NewFieldError("form.name", "等级名称不能为空。"))
	}
	payload["name"] = name

	level := util.ToIntDefault(payload["level"], 0)
	if level <= 0 {
		panic(frontaction.NewFieldError("form.level", "等级数字必须大于 0。"))
	}
	payload["level"] = level

	durationDays := util.ToIntDefault(payload["duration_days"], 0)
	if durationDays <= 0 {
		panic(frontaction.NewFieldError("form.duration_days", "时长天数必须大于 0。"))
	}
	payload["duration_days"] = durationDays

	durationType := util.ToIntDefault(payload["duration_type"], 0)
	if durationType != levelDurationReset && durationType != levelDurationRenew {
		panic(frontaction.NewFieldError("form.duration_type", "请选择升级时长类型。"))
	}
	payload["duration_type"] = durationType

	upgradeMethod := util.ToIntDefault(payload["upgrade_method"], 0)
	if upgradeMethod != levelUpgradePay && upgradeMethod != levelUpgradeRegister && upgradeMethod != levelUpgradeGift {
		panic(frontaction.NewFieldError("form.upgrade_method", "请选择升级方式。"))
	}
	payload["upgrade_method"] = upgradeMethod

	if upgradeMethod == levelUpgradePay {
		payType := util.ToIntDefault(payload["pay_type"], 0)
		if payType != levelPayDiff && payType != levelPayFull {
			panic(frontaction.NewFieldError("form.pay_type", "请选择支付方式。"))
		}
		payAmount := util.ToIntDefault(payload["pay_amount"], 0)
		if payAmount <= 0 {
			panic(frontaction.NewFieldError("form.pay_amount", "支付金额必须大于 0。"))
		}
		payload["pay_type"] = payType
		payload["pay_amount"] = payAmount
	} else {
		payload["pay_type"] = 0
		payload["pay_amount"] = 0
	}

	normalizePresentIdentitySortAndStatus(payload)
	return payload
}

func (UserHook) ProviderBeforeDeleteIdentityLevel(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	levelID := util.ToUint64(payload["id"])
	if levelID == 0 {
		panic("等级不存在")
	}
	if usermodel.NewUserIdentityModel().Count(c.Context(), map[string]any{"level_id": levelID}) > 0 {
		panic("当前等级已分配给用户，请先处理用户身份。")
	}
	if usermodel.NewUserBenefitGrantModel().Count(c.Context(), map[string]any{"level_id": levelID}) > 0 {
		panic("当前等级已有权益发放记录，不能删除。")
	}
	return map[string]any{"id": levelID}
}

func (UserHook) ProviderAfterSaveIdentity(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	identityID := util.ToUint64(payload["id"])
	if identityID == 0 {
		return payload
	}
	identityRow := usermodel.NewIdentityModel().FindMap(c.Context(), map[string]any{"id": identityID})
	if len(identityRow) == 0 {
		return payload
	}
	usermodel.NewUserIdentityModel().Update(c.Context(), map[string]any{
		"identity_id": identityID,
	}, map[string]any{
		"identity_name": strings.TrimSpace(util.ToString(identityRow["name"])),
	}, false)
	syncIdentityBenefitIdentitySnapshots(c, identityID, identityRow)
	return payload
}

func (UserHook) ProviderAfterSaveIdentityLevel(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	levelID := util.ToUint64(payload["id"])
	if levelID == 0 {
		return payload
	}
	levelRow := usermodel.NewIdentityLevelModel().FindMap(c.Context(), map[string]any{"id": levelID})
	if len(levelRow) == 0 {
		return payload
	}
	usermodel.NewUserIdentityModel().Update(c.Context(), map[string]any{
		"level_id": levelID,
	}, map[string]any{
		"level_name": strings.TrimSpace(util.ToString(levelRow["name"])),
		"level":      util.ToIntDefault(levelRow["level"], 0),
	}, false)
	syncIdentityBenefitLevelSnapshots(c, levelID, levelRow)
	return payload
}

func (UserHook) ProviderBeforeSaveUserIdentity(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	if isUserPartialRecord(payload) {
		normalizePresentIdentitySortAndStatus(payload)
		return payload
	}

	normalized := normalizeUserIdentityPayload(c, payload, false)
	if err := ensureUniqueUserIdentity(c, payload, normalized.userID, normalized.identityID); err != nil {
		panic(err)
	}

	payload["user_id"] = normalized.userID
	payload["user_name"] = strings.TrimSpace(util.ToString(normalized.userRow["name"]))
	payload["user_mobile"] = strings.TrimSpace(util.ToString(normalized.userRow["mobile"]))
	payload["identity_id"] = normalized.identityID
	payload["identity_name"] = strings.TrimSpace(util.ToString(normalized.identityRow["name"]))
	payload["level_id"] = normalized.levelID
	payload["level_name"] = strings.TrimSpace(util.ToString(normalized.levelRow["name"]))
	payload["level"] = util.ToIntDefault(normalized.levelRow["level"], 0)
	payload["status"] = normalized.status
	return payload
}

func (UserIdentityService) ProviderCreate(c *server.Context, params []any) any {
	payload := normalizeUserIdentityPayload(c, clonePointPayload(params), true)
	userIdentityModel := usermodel.NewUserIdentityModel()
	logModel := usermodel.NewUserIdentityLogModel()
	now := time.Now()
	var cardNo string
	var startedAt time.Time
	var expiredAt time.Time
	var userIdentityID uint64

	err := orm.Transaction(c.Context(), func(txCtx context.Context) error {
		existingRow := userIdentityModel.FindMap(txCtx, map[string]any{
			"user_id":     payload.userID,
			"identity_id": payload.identityID,
		})
		cardNo = userIdentityCardNo(txCtx, userIdentityModel, logModel, existingRow, now)
		startedAt, expiredAt = userIdentityPeriod(now, payload.levelRow, existingRow)

		record := payload.userIdentityRecord(cardNo, expiredAt)
		if len(existingRow) > 0 {
			userIdentityID = util.ToUint64(existingRow["id"])
			userIdentityModel.Update(txCtx, map[string]any{"id": userIdentityID}, record, false)
		} else {
			userIdentityID = util.ToUint64(userIdentityModel.Insert(txCtx, record))
		}
		if userIdentityID == 0 {
			return frontaction.NewFieldError("form.identity_id", "用户身份保存失败。")
		}

		logRecord := payload.userIdentityLogRecord(userIdentityID, cardNo, startedAt, expiredAt)
		logModel.Insert(txCtx, logRecord)
		return nil
	})
	if err != nil {
		panic(err)
	}

	return map[string]any{
		"id":          userIdentityID,
		"_virtual":    true,
		"user_id":     payload.userID,
		"status":      payload.status,
		"card_no":     cardNo,
		"expired_at":  expiredAt,
		"identity_id": payload.identityID,
		"level_id":    payload.levelID,
		"remark":      payload.remark,
	}
}

func isUserPartialRecord(payload map[string]any) bool {
	return util.ToBool(payload["_partial"])
}

func normalizePresentIdentitySortAndStatus(payload map[string]any) {
	if _, ok := payload["sort"]; ok {
		payload["sort"] = normalizeUserSort(payload["sort"])
	}
	if _, ok := payload["status"]; ok {
		payload["status"] = normalizeUserStatus(payload["status"])
	}
}

func normalizeUserSort(value any) int {
	sort := util.ToIntDefault(value, 100)
	if sort < 0 {
		return 0
	}
	return sort
}

func normalizeUserStatus(value any) int16 {
	status := util.ToIntDefault(value, identityStatusEnabled)
	if status == identityStatusDisabled {
		return identityStatusDisabled
	}
	return identityStatusEnabled
}

func ensureUniqueUserIdentity(c *server.Context, payload map[string]any, userID uint64, identityID uint64) error {
	userIdentityID := util.ToUint64(payload["id"])
	existingRow := usermodel.NewUserIdentityModel().FindMap(c.Context(), map[string]any{
		"user_id":     userID,
		"identity_id": identityID,
	})
	if len(existingRow) == 0 {
		return nil
	}
	if util.ToUint64(existingRow["id"]) == userIdentityID {
		return nil
	}
	return frontaction.NewFieldError("form.identity_id", "该用户已拥有当前身份，请编辑原记录。")
}

type userIdentityPayload struct {
	userID      uint64
	userRow     map[string]any
	identityID  uint64
	identityRow map[string]any
	levelID     uint64
	levelRow    map[string]any
	status      int16
	remark      string
}

func normalizeUserIdentityPayload(c *server.Context, payload map[string]any, requireRemark bool) userIdentityPayload {
	userID := util.ToUint64(payload["user_id"])
	if userID == 0 {
		panic(frontaction.NewFieldError("form.user_id", "用户不能为空。"))
	}
	userRow := usermodel.NewUserModel().FindMap(c.Context(), map[string]any{"id": userID})
	if len(userRow) == 0 {
		panic(frontaction.NewFieldError("form.user_id", "用户不存在。"))
	}

	levelID := util.ToUint64(payload["level_id"])
	if levelID == 0 {
		panic(frontaction.NewFieldError("form.level_id", "等级不能为空。"))
	}
	levelRow := usermodel.NewIdentityLevelModel().FindMap(c.Context(), map[string]any{"id": levelID})
	if len(levelRow) == 0 {
		panic(frontaction.NewFieldError("form.level_id", "等级不存在。"))
	}
	identityID := util.ToUint64(levelRow["identity_id"])
	if identityID == 0 {
		panic(frontaction.NewFieldError("form.level_id", "等级所属身份不存在。"))
	}
	identityRow := usermodel.NewIdentityModel().FindMap(c.Context(), map[string]any{"id": identityID})
	if len(identityRow) == 0 {
		panic(frontaction.NewFieldError("form.level_id", "身份不存在。"))
	}

	remark := strings.TrimSpace(util.ToString(payload["remark"]))
	if requireRemark && remark == "" {
		panic(frontaction.NewFieldError("form.remark", "请填写新增原因。"))
	}

	return userIdentityPayload{
		userID:      userID,
		userRow:     userRow,
		identityID:  identityID,
		identityRow: identityRow,
		levelID:     levelID,
		levelRow:    levelRow,
		status:      normalizeUserStatus(payload["status"]),
		remark:      remark,
	}
}

func (payload userIdentityPayload) userIdentityRecord(cardNo string, expiredAt time.Time) map[string]any {
	record := payload.userIdentitySnapshot(cardNo, expiredAt)
	record["status"] = payload.status
	return record
}

func (payload userIdentityPayload) userIdentityLogRecord(userIdentityID uint64, cardNo string, startedAt time.Time, expiredAt time.Time) map[string]any {
	record := payload.userIdentitySnapshot(cardNo, expiredAt)
	record["user_identity_id"] = userIdentityID
	record["started_at"] = startedAt
	record["remark"] = payload.remark
	return record
}

func (payload userIdentityPayload) userIdentitySnapshot(cardNo string, expiredAt time.Time) map[string]any {
	return map[string]any{
		"user_id":       payload.userID,
		"user_name":     strings.TrimSpace(util.ToString(payload.userRow["name"])),
		"user_mobile":   strings.TrimSpace(util.ToString(payload.userRow["mobile"])),
		"identity_id":   payload.identityID,
		"identity_name": strings.TrimSpace(util.ToString(payload.identityRow["name"])),
		"level_id":      payload.levelID,
		"level_name":    strings.TrimSpace(util.ToString(payload.levelRow["name"])),
		"level":         util.ToIntDefault(payload.levelRow["level"], 0),
		"card_no":       cardNo,
		"expired_at":    expiredAt,
	}
}

func userIdentityCardNo(ctx context.Context, userIdentityModel *orm.Model[usermodel.UserIdentity], logModel *orm.Model[usermodel.UserIdentityLog], existingRow map[string]any, now time.Time) string {
	cardNo := strings.TrimSpace(util.ToString(existingRow["card_no"]))
	if cardNo != "" {
		return cardNo
	}
	return generateUserIdentityCardNo(ctx, userIdentityModel, logModel, now)
}

func userIdentityPeriod(now time.Time, levelRow map[string]any, existingRow map[string]any) (time.Time, time.Time) {
	durationDays := util.ToIntDefault(levelRow["duration_days"], 0)
	if durationDays <= 0 {
		durationDays = 1
	}
	startedAt := now
	if util.ToIntDefault(levelRow["duration_type"], levelDurationReset) == levelDurationRenew {
		existingExpiredAt := normalizeUserIdentityTime(existingRow["expired_at"])
		if existingExpiredAt.After(startedAt) {
			startedAt = existingExpiredAt
		}
	}
	return startedAt, startedAt.AddDate(0, 0, durationDays)
}

func normalizeUserIdentityTime(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case string:
		return parseUserIdentityTime(typed)
	default:
		return time.Time{}
	}
}

func parseUserIdentityTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func generateUserIdentityCardNo(ctx context.Context, userIdentityModel *orm.Model[usermodel.UserIdentity], logModel *orm.Model[usermodel.UserIdentityLog], now time.Time) string {
	for attempt := 0; attempt < 5; attempt++ {
		cardNo := now.Format("20060102") + randomDigits(14)
		if len(userIdentityModel.FindMap(ctx, map[string]any{"card_no": cardNo})) == 0 &&
			len(logModel.FindMap(ctx, map[string]any{"card_no": cardNo})) == 0 {
			return cardNo
		}
	}
	return now.Format("20060102150405") + randomDigits(8)
}

func randomDigits(length int) string {
	if length <= 0 {
		return ""
	}
	max := big.NewInt(10)
	digits := make([]byte, 0, length)
	for len(digits) < length {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			digits = append(digits, byte('0'+time.Now().UnixNano()%10))
			continue
		}
		digits = append(digits, byte('0'+n.Int64()))
	}
	return string(digits)
}
