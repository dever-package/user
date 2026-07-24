package service

import (
	"strings"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	frontaction "github.com/dever-package/front/service/action"
	usermodel "github.com/dever-package/user/model"
)

type PointPackageService struct{}

func (PointPackageService) ProviderBeforeSave(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	if name, ok := payload["name"]; ok {
		payload["name"] = strings.TrimSpace(util.ToString(name))
		if payload["name"] == "" {
			panic(frontaction.NewFieldError("form.name", "套餐名称不能为空。"))
		}
	}
	if pointConfigID, ok := payload["point_config_id"]; ok {
		id := util.ToUint64(pointConfigID)
		if id == 0 || len(usermodel.NewPointConfigModel().FindMap(c.Context(), map[string]any{"id": id})) == 0 {
			panic(frontaction.NewFieldError("form.point_config_id", "请选择有效的积分配置。"))
		}
		payload["point_config_id"] = id
	}
	if value, ok := payload["point_amount"]; ok {
		amount := util.ToIntDefault(value, 0)
		if err := validatePointChangeAmount(amount, "form.point_amount"); err != nil {
			panic(err)
		}
		payload["point_amount"] = amount
	}
	if value, ok := payload["bonus_amount"]; ok {
		amount := util.ToIntDefault(value, 0)
		if amount < 0 || amount > maxPointChangeAmount {
			panic(frontaction.NewFieldError("form.bonus_amount", "赠送积分数量不正确。"))
		}
		payload["bonus_amount"] = amount
	}
	if util.ToIntDefault(payload["point_amount"], 0)+util.ToIntDefault(payload["bonus_amount"], 0) > maxPointChangeAmount {
		panic(frontaction.NewFieldError("form.bonus_amount", "套餐到账积分超过单次积分上限。"))
	}
	name := strings.TrimSpace(util.ToString(payload["name"]))
	pointConfigID := util.ToUint64(payload["point_config_id"])
	if name != "" && pointConfigID > 0 {
		existing := usermodel.NewPointPackageModel().FindMap(c.Context(), map[string]any{
			"point_config_id": pointConfigID, "name": name,
		})
		if existingID := util.ToUint64(existing["id"]); existingID > 0 && existingID != util.ToUint64(payload["id"]) {
			panic(frontaction.NewFieldError("form.name", "当前积分类型下已存在同名套餐。"))
		}
	}
	return payload
}
