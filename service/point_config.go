package service

import (
	"strings"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	frontaction "my/package/front/service/action"
	usermodel "my/package/user/model"
)

const (
	pointSymbolBefore = 1
	pointSymbolAfter  = 2
)

func (UserHook) ProviderBeforeSavePointConfig(_ *server.Context, params []any) any {
	payload := clonePointPayload(params)

	name := strings.TrimSpace(util.ToString(payload["name"]))
	if name == "" {
		panic(frontaction.NewFieldError("form.name", "积分名称不能为空。"))
	}
	payload["name"] = name

	payload["intro"] = strings.TrimSpace(util.ToString(payload["intro"]))
	exchangeRate := util.ToIntDefault(payload["exchange_rate"], 100)
	if exchangeRate < 0 {
		panic(frontaction.NewFieldError("form.exchange_rate", "货币换算不能小于 0。"))
	}
	payload["exchange_rate"] = exchangeRate
	payload["symbol"] = strings.TrimSpace(util.ToString(payload["symbol"]))
	payload["symbol_position"] = normalizePointSymbolPosition(payload["symbol_position"])
	return payload
}

func (UserHook) ProviderAfterSavePointConfig(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	pointConfigID := util.ToUint64(payload["id"])
	if pointConfigID == 0 {
		return payload
	}
	pointRow := usermodel.NewPointConfigModel().FindMap(c.Context(), map[string]any{"id": pointConfigID})
	if len(pointRow) == 0 {
		return payload
	}
	syncIdentityBenefitPointSnapshots(c, pointConfigID, pointRow)
	return payload
}

func (UserHook) ProviderBeforeDeletePointConfig(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	pointConfigID := util.ToUint64(payload["id"])
	if pointConfigID == 0 {
		panic("积分配置不存在")
	}
	if pointConfigID == 1 {
		panic("默认积分配置不能删除。")
	}
	if usermodel.NewIdentityModel().Count(c.Context(), map[string]any{"purchase_point_id": pointConfigID}) > 0 {
		panic("当前积分配置已用于身份购买积分，请先调整身份配置。")
	}
	if usermodel.NewUserPointModel().Count(c.Context(), map[string]any{"point_config_id": pointConfigID}) > 0 {
		panic("当前积分配置已有用户积分，请先处理用户积分。")
	}
	if usermodel.NewPointLogModel().Count(c.Context(), map[string]any{"point_config_id": pointConfigID}) > 0 {
		panic("当前积分配置已有积分日志，不能删除。")
	}
	if usermodel.NewIdentityBenefitModel().Count(c.Context(), map[string]any{"point_config_id": pointConfigID}) > 0 {
		panic("当前积分配置已用于身份权益，请先移除周期权益。")
	}
	if usermodel.NewUserBenefitGrantModel().Count(c.Context(), map[string]any{"point_config_id": pointConfigID}) > 0 {
		panic("当前积分配置已有权益发放记录，不能删除。")
	}
	return map[string]any{"id": pointConfigID}
}

func normalizePointSymbolPosition(value any) int16 {
	position := util.ToIntDefault(value, pointSymbolBefore)
	if position == pointSymbolAfter {
		return pointSymbolAfter
	}
	return pointSymbolBefore
}
