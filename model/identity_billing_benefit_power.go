package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type IdentityBillingBenefitPower struct {
	ID               uint64    `dorm:"primaryKey;autoIncrement;comment:计费权益能力ID"`
	BillingBenefitID uint64    `dorm:"type:bigint;not null;default:0;comment:计费权益"`
	PowerID          uint64    `dorm:"type:bigint;not null;default:0;comment:能力"`
	Sort             int       `dorm:"type:int;not null;default:100;comment:排序"`
	CreatedAt        time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type IdentityBillingBenefitPowerIndex struct {
	BenefitPower struct{} `unique:"billing_benefit_id,power_id"`
	BenefitSort  struct{} `index:"billing_benefit_id,sort,id"`
	PowerBenefit struct{} `index:"power_id,billing_benefit_id,id"`
}

func NewIdentityBillingBenefitPowerModel() *orm.Model[IdentityBillingBenefitPower] {
	return orm.LoadModel[IdentityBillingBenefitPower]("计费权益能力", "user_identity_billing_benefit_power", orm.ModelConfig{
		Index:    IdentityBillingBenefitPowerIndex{},
		Order:    "sort asc,id asc",
		Database: "default",
	})
}
