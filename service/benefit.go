package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	frontaction "github.com/dever-package/front/service/action"
	frontrecord "github.com/dever-package/front/service/record"
	usermodel "github.com/dever-package/user/model"
)

const (
	benefitTypeRewardPoint    = "reward_point"
	billingBenefitTypeAbility = "ability"
	maxBenefitCycleDays       = 36_500
	maxBenefitLimitTimes      = 1_000

	benefitClearEnabled  = 1
	benefitClearDisabled = 2
)

type BenefitService struct{}

func (BenefitService) ProviderBeforeSaveIdentityBenefit(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	if isUserPartialRecord(payload) {
		normalizeIdentityBenefitPartial(payload)
		return payload
	}

	levelID := identityBenefitLevelID(payload)
	if levelID == 0 {
		panic(frontaction.NewFieldError("form.level_id", "等级不能为空。"))
	}
	levelRow := usermodel.NewIdentityLevelModel().FindMap(c.Context(), map[string]any{"id": levelID})
	if len(levelRow) == 0 {
		panic(frontaction.NewFieldError("form.level_id", "等级不存在。"))
	}

	identityID := util.ToUint64(levelRow["identity_id"])
	identityRow := usermodel.NewIdentityModel().FindMap(c.Context(), map[string]any{"id": identityID})
	if len(identityRow) == 0 {
		panic(frontaction.NewFieldError("form.level_id", "等级所属身份不存在。"))
	}

	payload["identity_id"] = identityID
	payload["identity_name"] = strings.TrimSpace(util.ToString(identityRow["name"]))
	payload["level_id"] = levelID
	payload["level_name"] = strings.TrimSpace(util.ToString(levelRow["name"]))
	payload["level"] = util.ToIntDefault(levelRow["level"], 0)
	payload["periodic_benefits"] = normalizeIdentityBenefitRows(c.Context(), payload)
	payload["billing_benefits"] = normalizeIdentityBillingBenefitRows(c.Context(), payload)
	return payload
}

func (BenefitService) ProviderAttachIdentityBenefitForm(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	record := payload
	if loaded, ok := payload["record"].(map[string]any); ok {
		record = loaded
	}
	billingRows := normalizeBenefitChildRows(record["billing_benefits"])
	if len(billingRows) == 0 {
		return record
	}
	benefitIDs := make([]any, 0, len(billingRows))
	for _, row := range billingRows {
		if id := util.ToUint64(row["id"]); id > 0 {
			benefitIDs = append(benefitIDs, id)
		}
	}
	powerIDsByBenefit := map[uint64][]any{}
	if len(benefitIDs) > 0 {
		relations := usermodel.NewIdentityBillingBenefitPowerModel().SelectMap(c.Context(), map[string]any{
			"billing_benefit_id": benefitIDs,
		}, map[string]any{"order": "sort asc,id asc"})
		for _, relation := range relations {
			benefitID := util.ToUint64(relation["billing_benefit_id"])
			powerID := util.ToUint64(relation["power_id"])
			if benefitID == 0 || powerID == 0 {
				continue
			}
			powerIDsByBenefit[benefitID] = append(powerIDsByBenefit[benefitID], powerID)
		}
	}
	for _, row := range billingRows {
		row["power_ids"] = powerIDsByBenefit[util.ToUint64(row["id"])]
	}
	attachBillingPowerCateIDs(c.Context(), billingRows)
	record["billing_benefits"] = anyBenefitRows(billingRows)
	return record
}

func (BenefitService) ProviderAttachIdentityBenefitSummary(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	rows := normalizePointRows(payload["rows"])
	if len(rows) == 0 {
		return rows
	}

	levelIDs := collectIDsByField(rows, "id")
	if len(levelIDs) == 0 {
		return rows
	}

	benefitRows := usermodel.NewIdentityBenefitModel().SelectMap(c.Context(), map[string]any{
		"level_id": levelIDs,
	}, map[string]any{
		"order": "level_id asc,sort asc,id asc",
	})
	groupedBenefits := groupIdentityBenefitRows(benefitRows)
	billingRows := usermodel.NewIdentityBillingBenefitModel().SelectMap(c.Context(), map[string]any{
		"level_id": levelIDs,
	}, map[string]any{
		"order": "level_id asc,sort asc,id asc",
	})
	groupedBilling := groupIdentityBillingBenefitRows(billingRows)
	for _, row := range rows {
		levelID := util.ToUint64(row["id"])
		benefits := normalizeIdentityBenefitViewRows(groupedBenefits[levelID])
		row["periodic_benefits"] = formatIdentityBenefitSummary(benefits)
		row["billing_benefits"] = formatIdentityBillingBenefitSummary(groupedBilling[levelID])
	}
	return rows
}

func syncIdentityBenefitIdentitySnapshots(c *server.Context, identityID uint64, identityRow map[string]any) {
	if identityID == 0 || len(identityRow) == 0 {
		return
	}
	usermodel.NewIdentityBenefitModel().Update(c.Context(), map[string]any{
		"identity_id": identityID,
	}, map[string]any{
		"identity_name": strings.TrimSpace(util.ToString(identityRow["name"])),
	}, false)
	usermodel.NewIdentityBillingBenefitModel().Update(c.Context(), map[string]any{
		"identity_id": identityID,
	}, map[string]any{
		"identity_name": strings.TrimSpace(util.ToString(identityRow["name"])),
	}, false)
}

func syncIdentityBenefitLevelSnapshots(c *server.Context, levelID uint64, levelRow map[string]any) {
	if levelID == 0 || len(levelRow) == 0 {
		return
	}
	usermodel.NewIdentityBenefitModel().Update(c.Context(), map[string]any{
		"level_id": levelID,
	}, map[string]any{
		"level_name": strings.TrimSpace(util.ToString(levelRow["name"])),
		"level":      util.ToIntDefault(levelRow["level"], 0),
	}, false)
	usermodel.NewIdentityBillingBenefitModel().Update(c.Context(), map[string]any{
		"level_id": levelID,
	}, map[string]any{
		"level_name": strings.TrimSpace(util.ToString(levelRow["name"])),
		"level":      util.ToIntDefault(levelRow["level"], 0),
	}, false)
}

func syncIdentityBenefitPointSnapshots(c *server.Context, pointConfigID uint64, pointRow map[string]any) {
	if pointConfigID == 0 || len(pointRow) == 0 {
		return
	}
	pointSnapshot := pointConfigSnapshot(pointRow)
	usermodel.NewIdentityBenefitModel().Update(c.Context(), map[string]any{
		"point_config_id": pointConfigID,
	}, map[string]any{
		"point_name":            pointSnapshot.name,
		"point_symbol":          pointSnapshot.symbol,
		"point_symbol_position": pointSnapshot.symbolPosition,
	}, false)
	usermodel.NewIdentityBillingBenefitModel().Update(c.Context(), map[string]any{
		"point_config_id": pointConfigID,
	}, map[string]any{
		"point_name":            pointSnapshot.name,
		"point_symbol":          pointSnapshot.symbol,
		"point_symbol_position": pointSnapshot.symbolPosition,
	}, false)
}

func normalizeIdentityBenefitPartial(payload map[string]any) {
	if _, ok := payload["sort"]; ok {
		payload["sort"] = normalizeUserSort(payload["sort"])
	}
	if _, ok := payload["status"]; ok {
		payload["status"] = normalizeUserStatus(payload["status"])
	}
}

func normalizeIdentityBenefitRows(ctx context.Context, payload map[string]any) []any {
	levelID := util.ToUint64(payload["level_id"])
	identityID := util.ToUint64(payload["identity_id"])
	identityName := strings.TrimSpace(util.ToString(payload["identity_name"]))
	levelName := strings.TrimSpace(util.ToString(payload["level_name"]))
	level := util.ToIntDefault(payload["level"], 0)
	rows := normalizeBenefitChildRows(payload["periodic_benefits"])
	if len(rows) == 0 {
		return []any{}
	}

	seenBenefitKeys := map[string]struct{}{}
	result := make([]any, 0, len(rows))
	for index, row := range rows {
		next := normalizePeriodicBenefitDraft(ctx, row, index)
		pointConfigID := util.ToUint64(next["point_config_id"])
		cycleDays := util.ToIntDefault(next["cycle_days"], 1)
		benefitKey := util.ToString(pointConfigID) + ":" + util.ToString(cycleDays)
		if _, exists := seenBenefitKeys[benefitKey]; exists {
			panic(frontaction.NewFieldError("form.periodic_benefits", "同一积分同一天数不能重复配置。"))
		}
		seenBenefitKeys[benefitKey] = struct{}{}

		next["identity_id"] = identityID
		next["identity_name"] = identityName
		next["level_id"] = levelID
		next["level_name"] = levelName
		next["level"] = level
		result = append(result, next)
	}
	return result
}

func normalizeIdentityBillingBenefitRows(ctx context.Context, payload map[string]any) []any {
	levelID := util.ToUint64(payload["level_id"])
	identityID := util.ToUint64(payload["identity_id"])
	identityName := strings.TrimSpace(util.ToString(payload["identity_name"]))
	levelName := strings.TrimSpace(util.ToString(payload["level_name"]))
	level := util.ToIntDefault(payload["level"], 0)
	rows := normalizeBenefitChildRows(payload["billing_benefits"])
	if len(rows) == 0 {
		return []any{}
	}

	allConfigured := false
	seenPowerIDs := map[uint64]struct{}{}
	result := make([]any, 0, len(rows))
	for index, row := range rows {
		next := util.CloneMap(row)
		pointConfigID := util.ToUint64(next["point_config_id"])
		if pointConfigID == 0 {
			pointConfigID = defaultPointConfigID(ctx)
		}
		pointRow := usermodel.NewPointConfigModel().FindMap(ctx, map[string]any{"id": pointConfigID})
		if len(pointRow) == 0 {
			panic(frontaction.NewFieldError("form.billing_benefits", "计费权益选择的积分不存在。"))
		}
		if util.ToIntDefault(pointRow["exchange_rate"], 0) <= 0 {
			panic(frontaction.NewFieldError("form.billing_benefits", "计费权益选择的积分必须配置大于 0 的货币换算。"))
		}

		scope := normalizeBillingScope(next["scope"])
		powerIDs := normalizeBillingPowerIDs(next["power_ids"])
		if scope == usermodel.BillingScopeAll {
			if allConfigured {
				panic(frontaction.NewFieldError("form.billing_benefits", "同一身份等级只能配置一条全部能力计费权益。"))
			}
			allConfigured = true
			powerIDs = nil
		} else {
			powerCateID := util.ToUint64(next["power_cate_id"])
			if powerCateID == 0 {
				panic(frontaction.NewFieldError("form.billing_benefits", "指定能力计费权益必须选择能力分类。"))
			}
			if len(powerIDs) == 0 {
				panic(frontaction.NewFieldError("form.billing_benefits", "指定能力计费权益必须至少选择一个能力。"))
			}
			validateBillingPowerSelection(ctx, powerCateID, powerIDs)
			for _, powerID := range powerIDs {
				if _, exists := seenPowerIDs[powerID]; exists {
					panic(frontaction.NewFieldError("form.billing_benefits", "同一个能力不能在当前等级的多条计费权益中重复配置。"))
				}
				seenPowerIDs[powerID] = struct{}{}
			}
		}

		ratioBasisPoints, err := ParseSaleRatioBasisPoints(next["sale_ratio"])
		if err != nil {
			panic(frontaction.NewFieldError("form.billing_benefits", err.Error()))
		}
		pointSnapshot := pointConfigSnapshot(pointRow)
		next["identity_id"] = identityID
		next["identity_name"] = identityName
		next["level_id"] = levelID
		next["level_name"] = levelName
		next["level"] = level
		next["point_config_id"] = pointConfigID
		next["point_name"] = pointSnapshot.name
		next["point_symbol"] = pointSnapshot.symbol
		next["point_symbol_position"] = pointSnapshot.symbolPosition
		next["scope"] = scope
		next["sale_ratio"] = FormatSaleRatio(ratioBasisPoints)
		next["power_ids"] = uint64Values(powerIDs)
		next["status"] = normalizeUserStatus(next["status"])
		next["sort"] = normalizeBenefitSort(next["sort"], index)
		delete(next, "power_cate_id")
		result = append(result, next)
	}
	return result
}

func ParseSaleRatioBasisPoints(value any) (int64, error) {
	raw := strings.TrimSpace(util.ToString(value))
	if raw == "" {
		raw = "1"
	}
	if strings.HasPrefix(raw, "+") {
		raw = strings.TrimPrefix(raw, "+")
	}
	if strings.HasPrefix(raw, "-") {
		return 0, fmt.Errorf("售价系数不能小于 0")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" || !billingDigits(parts[0]) {
		return 0, fmt.Errorf("售价系数格式无效")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" || len(fraction) > 4 || !billingDigits(fraction) {
			return 0, fmt.Errorf("售价系数最多支持 4 位小数")
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 1000 {
		return 0, fmt.Errorf("售价系数不能超过 1000")
	}
	for len(fraction) < 4 {
		fraction += "0"
	}
	fractionValue := int64(0)
	if fraction != "" {
		fractionValue, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("售价系数格式无效")
		}
	}
	basisPoints := whole*10000 + fractionValue
	if basisPoints > 10_000_000 {
		return 0, fmt.Errorf("售价系数不能超过 1000")
	}
	return basisPoints, nil
}

func FormatSaleRatio(basisPoints int64) string {
	if basisPoints <= 0 {
		return "0"
	}
	whole := basisPoints / 10000
	fraction := basisPoints % 10000
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	return strconv.FormatInt(whole, 10) + "." + strings.TrimRight(fmt.Sprintf("%04d", fraction), "0")
}

func normalizeBillingScope(value any) string {
	if strings.TrimSpace(util.ToString(value)) == usermodel.BillingScopeSpecified {
		return usermodel.BillingScopeSpecified
	}
	return usermodel.BillingScopeAll
}

func normalizeBillingPowerIDs(value any) []uint64 {
	values := []any{}
	switch current := value.(type) {
	case []any:
		values = current
	case []uint64:
		result := make([]uint64, 0, len(current))
		for _, id := range current {
			if id > 0 {
				result = appendUniqueUint64(result, id)
			}
		}
		return result
	case nil:
		return nil
	default:
		values = append(values, current)
	}
	result := make([]uint64, 0, len(values))
	for _, item := range values {
		id := util.ToUint64(item)
		if row, ok := item.(map[string]any); ok {
			id = util.ToUint64(row["id"])
		}
		if id > 0 {
			result = appendUniqueUint64(result, id)
		}
	}
	return result
}

func validateBillingPowerSelection(ctx context.Context, powerCateID uint64, powerIDs []uint64) {
	categoryModel := frontrecord.Resolve("bot.energon.NewPowerCateModel")
	if categoryModel == nil {
		panic(frontaction.NewFieldError("form.billing_benefits", "能力分类模型未加载，无法保存指定能力计费权益。"))
	}
	if len(categoryModel.FindMap(ctx, map[string]any{
		"id":     powerCateID,
		"status": identityStatusEnabled,
	})) == 0 {
		panic(frontaction.NewFieldError("form.billing_benefits", "选择的能力分类已停用或不存在。"))
	}

	powerModel := frontrecord.Resolve("bot.energon.NewPowerModel")
	if powerModel == nil {
		panic(frontaction.NewFieldError("form.billing_benefits", "能力模型未加载，无法保存指定能力计费权益。"))
	}
	filters := make([]any, 0, len(powerIDs))
	for _, powerID := range powerIDs {
		filters = append(filters, powerID)
	}
	rows := powerModel.SelectMap(ctx, map[string]any{
		"id":      filters,
		"cate_id": powerCateID,
		"status":  identityStatusEnabled,
	}, map[string]any{"field": "main.id"})
	if len(rows) != len(powerIDs) {
		panic(frontaction.NewFieldError("form.billing_benefits", "指定能力中存在不属于所选分类、已停用或不存在的能力。"))
	}
}

func attachBillingPowerCateIDs(ctx context.Context, billingRows []map[string]any) {
	if len(billingRows) == 0 {
		return
	}
	powerIDs := make([]uint64, 0)
	for _, row := range billingRows {
		for _, powerID := range normalizeBillingPowerIDs(row["power_ids"]) {
			powerIDs = appendUniqueUint64(powerIDs, powerID)
		}
	}
	if len(powerIDs) == 0 {
		return
	}
	powerModel := frontrecord.Resolve("bot.energon.NewPowerModel")
	if powerModel == nil {
		return
	}
	filters := make([]any, 0, len(powerIDs))
	for _, powerID := range powerIDs {
		filters = append(filters, powerID)
	}
	powers := powerModel.SelectMap(ctx, map[string]any{"id": filters}, map[string]any{
		"field": "main.id,main.cate_id",
	})
	cateByPower := make(map[uint64]uint64, len(powers))
	for _, power := range powers {
		cateByPower[util.ToUint64(power["id"])] = util.ToUint64(power["cate_id"])
	}
	for _, row := range billingRows {
		row["power_cate_id"] = commonBillingPowerCateID(normalizeBillingPowerIDs(row["power_ids"]), cateByPower)
	}
}

func commonBillingPowerCateID(powerIDs []uint64, cateByPower map[uint64]uint64) uint64 {
	var commonCateID uint64
	for _, powerID := range powerIDs {
		cateID := cateByPower[powerID]
		if cateID == 0 {
			return 0
		}
		if commonCateID == 0 {
			commonCateID = cateID
			continue
		}
		if commonCateID != cateID {
			return 0
		}
	}
	return commonCateID
}

func uint64Values(values []uint64) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func appendUniqueUint64(values []uint64, value uint64) []uint64 {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func billingDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, current := range value {
		if current < '0' || current > '9' {
			return false
		}
	}
	return true
}

func anyBenefitRows(rows []map[string]any) []any {
	result := make([]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, row)
	}
	return result
}

func normalizePeriodicBenefitDraft(ctx context.Context, row map[string]any, index int) map[string]any {
	next := util.CloneMap(row)
	benefitType := normalizeBenefitType(next["benefit_type"])
	if benefitType == "" {
		benefitType = benefitTypeRewardPoint
	}
	if benefitType == "" {
		panic(frontaction.NewFieldError("form.periodic_benefits", "请选择权益类型。"))
	}
	pointConfigID := util.ToUint64(next["point_config_id"])
	if pointConfigID == 0 {
		pointConfigID = defaultPointConfigID(ctx)
	}
	pointRow := usermodel.NewPointConfigModel().FindMap(ctx, map[string]any{"id": pointConfigID})
	if len(pointRow) == 0 {
		panic(frontaction.NewFieldError("form.periodic_benefits", "奖励积分不存在。"))
	}

	pointAmount := util.ToIntDefault(next["point_amount"], 0)
	if err := validatePointChangeAmount(pointAmount, "form.periodic_benefits"); err != nil {
		panic(err)
	}
	cycleDays := normalizeBenefitPositiveInt(next["cycle_days"], 1)
	if cycleDays <= 0 {
		panic(frontaction.NewFieldError("form.periodic_benefits", "发放天数必须大于 0。"))
	}
	if cycleDays > maxBenefitCycleDays {
		panic(frontaction.NewFieldError("form.periodic_benefits", "发放天数不能超过 36500 天。"))
	}
	limitTimes := normalizeBenefitPositiveInt(next["limit_times"], 1)
	if limitTimes <= 0 {
		panic(frontaction.NewFieldError("form.periodic_benefits", "上限次数必须大于 0。"))
	}
	if limitTimes > maxBenefitLimitTimes {
		panic(frontaction.NewFieldError("form.periodic_benefits", "上限次数不能超过 1000 次。"))
	}

	pointSnapshot := pointConfigSnapshot(pointRow)
	next["benefit_type"] = benefitType
	next["point_config_id"] = pointConfigID
	next["point_name"] = pointSnapshot.name
	next["point_symbol"] = pointSnapshot.symbol
	next["point_symbol_position"] = pointSnapshot.symbolPosition
	next["point_amount"] = pointAmount
	next["cycle_days"] = cycleDays
	next["limit_times"] = limitTimes
	next["clear_previous"] = normalizeBenefitClearPrevious(next["clear_previous"])
	next["status"] = normalizeUserStatus(next["status"])
	next["sort"] = normalizeBenefitSort(next["sort"], index)
	return next
}

func identityBenefitLevelID(payload map[string]any) uint64 {
	if levelID := util.ToUint64(payload["level_id"]); levelID > 0 {
		return levelID
	}
	return util.ToUint64(payload["id"])
}

func normalizeBenefitChildRows(value any) []map[string]any {
	switch current := value.(type) {
	case []map[string]any:
		return current
	case []any:
		rows := make([]map[string]any, 0, len(current))
		for _, item := range current {
			row, ok := item.(map[string]any)
			if ok {
				rows = append(rows, row)
			}
		}
		return rows
	default:
		return nil
	}
}

func normalizeBenefitSort(value any, index int) int {
	sort := util.ToIntDefault(value, 0)
	if sort > 0 {
		return sort
	}
	return index + 1
}

func normalizeBenefitType(value any) string {
	switch strings.TrimSpace(util.ToString(value)) {
	case benefitTypeRewardPoint:
		return benefitTypeRewardPoint
	default:
		return ""
	}
}

func normalizeBenefitPositiveInt(value any, defaultValue int) int {
	next := util.ToIntDefault(value, defaultValue)
	if next < 0 {
		return 0
	}
	return next
}

func normalizeBenefitClearPrevious(value any) int16 {
	if util.ToIntDefault(value, benefitClearEnabled) == benefitClearDisabled {
		return benefitClearDisabled
	}
	return benefitClearEnabled
}

func groupIdentityBenefitRows(rows []map[string]any) map[uint64][]map[string]any {
	grouped := map[uint64][]map[string]any{}
	for _, row := range rows {
		levelID := util.ToUint64(row["level_id"])
		if levelID == 0 {
			continue
		}
		grouped[levelID] = append(grouped[levelID], row)
	}
	return grouped
}

func groupIdentityBillingBenefitRows(rows []map[string]any) map[uint64][]map[string]any {
	grouped := map[uint64][]map[string]any{}
	for _, row := range rows {
		levelID := util.ToUint64(row["level_id"])
		if levelID == 0 {
			continue
		}
		grouped[levelID] = append(grouped[levelID], row)
	}
	return grouped
}

func formatIdentityBillingBenefitSummary(rows []map[string]any) string {
	if len(rows) == 0 {
		return "无"
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		if normalizeUserStatus(row["status"]) != identityStatusEnabled {
			continue
		}
		scope := "全部能力"
		if normalizeBillingScope(row["scope"]) == usermodel.BillingScopeSpecified {
			scope = "指定能力"
		}
		pointName := strings.TrimSpace(util.ToString(row["point_name"]))
		if pointName == "" {
			pointName = "积分"
		}
		parts = append(parts, scope+" · "+strings.TrimSpace(util.ToString(row["sale_ratio"]))+" 倍 · "+pointName)
	}
	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, "；")
}

func normalizeIdentityBenefitViewRows(rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return rows
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := util.CloneMap(row)
		if normalizeBenefitType(next["benefit_type"]) == "" {
			next["benefit_type"] = benefitTypeRewardPoint
		}
		result = append(result, next)
	}
	return result
}

func formatIdentityBenefitSummary(rows []map[string]any) string {
	if len(rows) == 0 {
		return "无"
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		if normalizeUserStatus(row["status"]) != identityStatusEnabled {
			continue
		}
		amount := util.ToIntDefault(row["point_amount"], 0)
		if amount <= 0 {
			continue
		}
		pointName := strings.TrimSpace(util.ToString(row["point_name"]))
		if pointName == "" {
			pointName = "积分"
		}
		cycleDays := normalizeBenefitPositiveInt(row["cycle_days"], 1)
		limitTimes := normalizeBenefitPositiveInt(row["limit_times"], 1)
		parts = append(parts, "每隔 "+util.ToString(cycleDays)+" 天发放 "+
			util.ToString(amount)+pointName+
			"，上限 "+util.ToString(limitTimes)+" 次"+
			"，"+formatBenefitClearPreviousName(row["clear_previous"]))
	}
	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, "；")
}

func formatBenefitClearPreviousName(value any) string {
	if normalizeBenefitClearPrevious(value) == benefitClearDisabled {
		return "不清空上次权益"
	}
	return "清空上次权益"
}
