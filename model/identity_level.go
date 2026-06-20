package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type IdentityLevel struct {
	ID            uint64    `dorm:"primaryKey;autoIncrement;comment:等级ID"`
	IdentityID    uint64    `dorm:"type:bigint;not null;comment:所属身份"`
	Name          string    `dorm:"type:varchar(64);not null;comment:等级名称"`
	Level         int       `dorm:"type:int;not null;default:1;comment:等级数字"`
	DurationDays  int       `dorm:"type:int;not null;default:0;comment:时长天数"`
	DurationType  int16     `dorm:"type:smallint;not null;default:1;comment:升级时长类型"`
	UpgradeMethod int16     `dorm:"type:smallint;not null;default:1;comment:升级方式"`
	PayType       int16     `dorm:"type:smallint;not null;default:0;comment:支付方式"`
	PayAmount     int       `dorm:"type:int;not null;default:0;comment:支付金额"`
	Status        int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	Sort          int       `dorm:"type:int;not null;default:100;comment:排序"`
	CreatedAt     time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type IdentityLevelIndex struct {
	IdentityLevel      struct{} `unique:"identity_id,level"`
	IdentityStatusSort struct{} `index:"identity_id,status,sort,id"`
	UpgradeMethod      struct{} `index:"upgrade_method,id"`
	CreatedAt          struct{} `index:"created_at"`
}

var identityLevelIdentityRelation = orm.Relation{
	Field:      "identity_id",
	Name:       "identity",
	Option:     "user.NewIdentityModel",
	OptionKeys: []string{"name", "status"},
}

var identityLevelBenefitRelation = orm.Relation{
	Field:      "periodic_benefits",
	Through:    "user.NewIdentityBenefitModel",
	OwnerField: "level_id",
	Order:      "sort asc,id asc",
}

func NewIdentityLevelModel() *orm.Model[IdentityLevel] {
	return orm.LoadModel[IdentityLevel]("身份等级", "user_identity_level", orm.ModelConfig{
		Index:    IdentityLevelIndex{},
		Order:    "identity_id asc,sort asc,level asc,id asc",
		Database: "default",
		Options: map[string]any{
			"duration_type":  levelDurationTypeOptions,
			"upgrade_method": levelUpgradeMethodOptions,
			"pay_type":       levelPayTypeOptions,
			"status":         identityStatusOptions,
		},
		Relations: []orm.Relation{
			identityLevelIdentityRelation,
			identityLevelBenefitRelation,
		},
	})
}
