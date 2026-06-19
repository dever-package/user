package service

import (
	"context"
	"strings"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	frontaction "github.com/dever-package/front/service/action"
	usermodel "github.com/dever-package/user/model"
)

const (
	benefitTypeRewardPoint = "reward_point"

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
	return payload
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
	for _, row := range rows {
		levelID := util.ToUint64(row["id"])
		benefits := normalizeIdentityBenefitViewRows(groupedBenefits[levelID])
		row["periodic_benefits"] = formatIdentityBenefitSummary(benefits)
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
	if pointAmount <= 0 {
		panic(frontaction.NewFieldError("form.periodic_benefits", "奖励数量必须大于 0。"))
	}
	cycleDays := normalizeBenefitPositiveInt(next["cycle_days"], 1)
	if cycleDays <= 0 {
		panic(frontaction.NewFieldError("form.periodic_benefits", "发放天数必须大于 0。"))
	}
	limitTimes := normalizeBenefitPositiveInt(next["limit_times"], 1)
	if limitTimes <= 0 {
		panic(frontaction.NewFieldError("form.periodic_benefits", "上限次数必须大于 0。"))
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
