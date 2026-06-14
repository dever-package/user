package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type Identity struct {
	ID              uint64    `dorm:"primaryKey;autoIncrement;comment:身份ID"`
	Name            string    `dorm:"type:varchar(64);not null;comment:身份名称"`
	PurchasePointID uint64    `dorm:"type:bigint;not null;default:1;comment:购买积分"`
	Status          int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	Sort            int       `dorm:"type:int;not null;default:100;comment:排序"`
	CreatedAt       time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type IdentityIndex struct {
	Name          struct{} `unique:"name"`
	PurchasePoint struct{} `index:"purchase_point_id,status"`
	StatusSort    struct{} `index:"status,sort,id"`
	CreatedAt     struct{} `index:"created_at"`
}

var identityPurchasePointRelation = orm.Relation{
	Field:      "purchase_point_id",
	Name:       "purchase_point",
	Option:     "user.NewPointConfigModel",
	OptionKeys: []string{"name", "exchange_rate", "symbol", "symbol_position"},
}

func NewIdentityModel() *orm.Model[Identity] {
	return orm.LoadModel[Identity]("身份", "identity", orm.ModelConfig{
		Index:    IdentityIndex{},
		Order:    "sort asc,id asc",
		Database: "default",
		Options: map[string]any{
			"status": identityStatusOptions,
		},
		Relations: []orm.Relation{
			identityPurchasePointRelation,
		},
	})
}
