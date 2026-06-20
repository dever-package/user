package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type PointConfig struct {
	ID             uint64    `dorm:"primaryKey;autoIncrement;comment:配置ID"`
	Name           string    `dorm:"type:varchar(64);not null;comment:积分名称"`
	Intro          string    `dorm:"type:text;not null;default:'';comment:积分介绍"`
	ExchangeRate   int       `dorm:"type:int;not null;default:100;comment:货币换算"`
	Symbol         string    `dorm:"type:varchar(16);not null;default:'';comment:积分符号"`
	SymbolPosition int16     `dorm:"type:smallint;not null;default:1;comment:符号位置"`
	CreatedAt      time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

var pointConfigSeed = []map[string]any{
	{
		"id":              1,
		"name":            "积分",
		"intro":           "",
		"exchange_rate":   100,
		"symbol":          "积分",
		"symbol_position": 2,
	},
}

func NewPointConfigModel() *orm.Model[PointConfig] {
	return orm.LoadModel[PointConfig]("积分配置", "user_point_config", orm.ModelConfig{
		Seeds:    pointConfigSeed,
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"symbol_position": pointSymbolPositionOptions,
		},
	})
}
