package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type PointPackage struct {
	ID            uint64    `dorm:"primaryKey;autoIncrement;comment:套餐ID"`
	PointConfigID uint64    `dorm:"type:bigint;not null;default:1;comment:积分类型"`
	Name          string    `dorm:"type:varchar(64);not null;comment:套餐名称"`
	PointAmount   int       `dorm:"type:int;not null;default:0;comment:基础积分"`
	BonusAmount   int       `dorm:"type:int;not null;default:0;comment:赠送积分"`
	Status        int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	Sort          int       `dorm:"type:int;not null;default:100;comment:排序"`
	CreatedAt     time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type PointPackageIndex struct {
	PointName   struct{} `unique:"point_config_id,name"`
	StatusSort  struct{} `index:"status,sort,id"`
	PointStatus struct{} `index:"point_config_id,status,sort,id"`
	CreatedAt   struct{} `index:"created_at"`
}

var pointPackageConfigRelation = orm.Relation{
	Field:      "point_config_id",
	Name:       "point_config",
	Option:     "user.NewPointConfigModel",
	OptionKeys: []string{"name", "exchange_rate", "symbol", "symbol_position"},
}

func NewPointPackageModel() *orm.Model[PointPackage] {
	return orm.LoadModel[PointPackage]("积分套餐", "user_point_package", orm.ModelConfig{
		Index:    PointPackageIndex{},
		Order:    "sort asc,id asc",
		Database: "default",
		Options: map[string]any{
			"status": identityStatusOptions,
		},
		Relations: []orm.Relation{pointPackageConfigRelation},
	})
}
