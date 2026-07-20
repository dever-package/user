package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

const (
	BillingScopeAll       = "all"
	BillingScopeSpecified = "specified"
)

type IdentityBillingBenefit struct {
	ID                  uint64    `dorm:"primaryKey;autoIncrement;comment:计费权益ID"`
	IdentityID          uint64    `dorm:"type:bigint;not null;comment:身份"`
	IdentityName        string    `dorm:"type:varchar(64);not null;default:'';comment:身份名称"`
	LevelID             uint64    `dorm:"type:bigint;not null;comment:等级"`
	LevelName           string    `dorm:"type:varchar(64);not null;default:'';comment:等级名称"`
	Level               int       `dorm:"type:int;not null;default:0;comment:等级数字"`
	PointConfigID       uint64    `dorm:"type:bigint;not null;default:1;comment:消费积分"`
	PointName           string    `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string    `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16     `dorm:"type:smallint;not null;default:2;comment:符号位置"`
	Scope               string    `dorm:"type:varchar(16);not null;default:'all';comment:适用范围"`
	SaleRatio           string    `dorm:"type:varchar(24);not null;default:'1';comment:售价系数"`
	Status              int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	Sort                int       `dorm:"type:int;not null;default:100;comment:排序"`
	CreatedAt           time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type IdentityBillingBenefitIndex struct {
	IdentityLevel  struct{} `index:"identity_id,level_id,status,sort,id"`
	LevelCreatedAt struct{} `index:"level_id,created_at,id"`
	PointCreatedAt struct{} `index:"point_config_id,created_at,id"`
	ScopeStatus    struct{} `index:"scope,status,id"`
	CreatedAt      struct{} `index:"created_at"`
}

var (
	identityBillingScopeOptions = []map[string]any{
		{"id": BillingScopeAll, "value": "全部能力"},
		{"id": BillingScopeSpecified, "value": "指定能力"},
	}

	identityBillingIdentityRelation = orm.Relation{
		Field:      "identity_id",
		Name:       "identity",
		Option:     "user.NewIdentityModel",
		OptionKeys: []string{"name", "status"},
	}

	identityBillingLevelRelation = orm.Relation{
		Field:      "level_id",
		Name:       "identity_level",
		Option:     "user.NewIdentityLevelModel",
		OptionKeys: []string{"name", "level", "identity_id", "status"},
	}

	identityBillingPointRelation = orm.Relation{
		Field:      "point_config_id",
		Name:       "point_config",
		Option:     "user.NewPointConfigModel",
		OptionKeys: []string{"name", "exchange_rate", "symbol", "symbol_position"},
	}

	identityBillingPowerRelation = orm.Relation{
		Field:        "power_ids",
		Name:         "powers",
		Through:      "user.NewIdentityBillingBenefitPowerModel",
		Option:       "bot.energon.NewPowerModel",
		OwnerField:   "billing_benefit_id",
		TargetField:  "power_id",
		ThroughOrder: "sort asc,id asc",
		OptionKeys:   []string{"name", "key", "cate_id", "kind", "status"},
	}
)

func NewIdentityBillingBenefitModel() *orm.Model[IdentityBillingBenefit] {
	return orm.LoadModel[IdentityBillingBenefit]("计费权益", "user_identity_billing_benefit", orm.ModelConfig{
		Index:    IdentityBillingBenefitIndex{},
		Order:    "identity_id asc,level asc,sort asc,id asc",
		Database: "default",
		Options: map[string]any{
			"scope":                 identityBillingScopeOptions,
			"status":                identityStatusOptions,
			"point_symbol_position": pointSymbolPositionOptions,
		},
		Relations: []orm.Relation{
			identityBillingIdentityRelation,
			identityBillingLevelRelation,
			identityBillingPointRelation,
			identityBillingPowerRelation,
		},
	})
}
