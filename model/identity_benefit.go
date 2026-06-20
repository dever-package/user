package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type IdentityBenefit struct {
	ID                  uint64    `dorm:"primaryKey;autoIncrement;comment:权益ID"`
	IdentityID          uint64    `dorm:"type:bigint;not null;comment:身份"`
	IdentityName        string    `dorm:"type:varchar(64);not null;default:'';comment:身份名称"`
	LevelID             uint64    `dorm:"type:bigint;not null;comment:等级"`
	LevelName           string    `dorm:"type:varchar(64);not null;default:'';comment:等级名称"`
	Level               int       `dorm:"type:int;not null;default:0;comment:等级数字"`
	BenefitType         string    `dorm:"type:varchar(32);not null;default:'reward_point';comment:权益类型"`
	PointConfigID       uint64    `dorm:"type:bigint;not null;default:1;comment:奖励积分"`
	PointName           string    `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string    `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16     `dorm:"type:smallint;not null;default:2;comment:符号位置"`
	PointAmount         int       `dorm:"type:int;not null;default:0;comment:奖励数量"`
	CycleDays           int       `dorm:"type:int;not null;default:1;comment:发放天数"`
	LimitTimes          int       `dorm:"type:int;not null;default:1;comment:上限次数"`
	ClearPrevious       int16     `dorm:"type:smallint;not null;default:1;comment:清空上次权益"`
	Status              int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	Sort                int       `dorm:"type:int;not null;default:100;comment:排序"`
	CreatedAt           time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type IdentityBenefitIndex struct {
	IdentityLevelBenefit struct{} `unique:"identity_id,level_id,benefit_type,point_config_id,cycle_days"`
	IdentityLevel        struct{} `index:"identity_id,level_id,status,sort,id"`
	LevelCreatedAt       struct{} `index:"level_id,created_at,id"`
	PointConfigCreatedAt struct{} `index:"point_config_id,created_at,id"`
	StatusSort           struct{} `index:"status,sort,id"`
	CreatedAt            struct{} `index:"created_at"`
}

var identityBenefitIdentityRelation = orm.Relation{
	Field:      "identity_id",
	Name:       "identity",
	Option:     "user.NewIdentityModel",
	OptionKeys: []string{"name", "status"},
}

var identityBenefitLevelRelation = orm.Relation{
	Field:      "level_id",
	Name:       "identity_level",
	Option:     "user.NewIdentityLevelModel",
	OptionKeys: []string{"name", "level", "identity_id", "status"},
}

var identityBenefitPointRelation = orm.Relation{
	Field:      "point_config_id",
	Name:       "point_config",
	Option:     "user.NewPointConfigModel",
	OptionKeys: []string{"name", "symbol", "symbol_position"},
}

func NewIdentityBenefitModel() *orm.Model[IdentityBenefit] {
	return orm.LoadModel[IdentityBenefit]("身份权益", "user_identity_benefit", orm.ModelConfig{
		Index:    IdentityBenefitIndex{},
		Order:    "identity_id asc,level asc,sort asc,id asc",
		Database: "default",
		Options: map[string]any{
			"benefit_type":          benefitTypeOptions,
			"clear_previous":        benefitClearPreviousOptions,
			"status":                identityStatusOptions,
			"point_symbol_position": pointSymbolPositionOptions,
		},
		Relations: []orm.Relation{
			identityBenefitIdentityRelation,
			identityBenefitLevelRelation,
			identityBenefitPointRelation,
		},
	})
}
