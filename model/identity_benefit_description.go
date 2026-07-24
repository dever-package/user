package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type IdentityBenefitDescription struct {
	ID        uint64    `dorm:"primaryKey;autoIncrement;comment:权益描述ID"`
	LevelID   uint64    `dorm:"type:bigint;not null;comment:等级"`
	Icon      string    `dorm:"type:varchar(64);not null;default:'circle-check';comment:图标"`
	Content   string    `dorm:"type:varchar(200);not null;comment:文案"`
	Sort      int       `dorm:"type:int;not null;default:100;comment:排序"`
	CreatedAt time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type IdentityBenefitDescriptionIndex struct {
	LevelSort struct{} `index:"level_id,sort,id"`
}

func NewIdentityBenefitDescriptionModel() *orm.Model[IdentityBenefitDescription] {
	return orm.LoadModel[IdentityBenefitDescription]("身份权益描述", "user_identity_benefit_description", orm.ModelConfig{
		Index:    IdentityBenefitDescriptionIndex{},
		Order:    "level_id asc,sort asc,id asc",
		Database: "default",
	})
}
