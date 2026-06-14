package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type UserBenefitGrant struct {
	ID                  uint64     `dorm:"primaryKey;autoIncrement;comment:权益发放ID"`
	UserIdentityID      uint64     `dorm:"type:bigint;not null;default:0;comment:用户身份"`
	UserID              uint64     `dorm:"type:bigint;not null;default:0;comment:用户"`
	UserName            string     `dorm:"type:varchar(64);not null;default:'';comment:姓名"`
	UserMobile          string     `dorm:"type:varchar(32);not null;default:'';comment:手机号"`
	IdentityBenefitID   uint64     `dorm:"type:bigint;not null;default:0;comment:身份权益"`
	IdentityID          uint64     `dorm:"type:bigint;not null;default:0;comment:身份"`
	IdentityName        string     `dorm:"type:varchar(64);not null;default:'';comment:身份名称"`
	LevelID             uint64     `dorm:"type:bigint;not null;default:0;comment:等级"`
	LevelName           string     `dorm:"type:varchar(64);not null;default:'';comment:等级名称"`
	Level               int        `dorm:"type:int;not null;default:1;comment:等级数字"`
	PointConfigID       uint64     `dorm:"type:bigint;not null;default:1;comment:积分"`
	PointName           string     `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string     `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16      `dorm:"type:smallint;not null;default:2;comment:符号位置"`
	Amount              int        `dorm:"type:int;not null;default:0;comment:发放积分"`
	RemainingAmount     int        `dorm:"type:int;not null;default:0;comment:剩余积分"`
	CycleStartAt        time.Time  `dorm:"not null;default:CURRENT_TIMESTAMP;comment:周期开始时间"`
	CycleEndAt          time.Time  `dorm:"not null;default:CURRENT_TIMESTAMP;comment:周期结束时间"`
	GrantNo             int        `dorm:"type:int;not null;default:1;comment:周期内发放序号"`
	Status              int16      `dorm:"type:smallint;not null;default:1;comment:状态"`
	ClearedAt           *time.Time `dorm:"null;comment:清空时间"`
	CreatedAt           time.Time  `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type UserBenefitGrantIndex struct {
	GrantCycle             struct{} `unique:"user_identity_id,identity_benefit_id,cycle_start_at,grant_no"`
	UserPointStatus        struct{} `index:"user_id,point_config_id,status,created_at,id"`
	UserIdentityBenefit    struct{} `index:"user_identity_id,identity_benefit_id,status,created_at,id"`
	IdentityBenefitCreated struct{} `index:"identity_benefit_id,created_at,id"`
	CycleStatus            struct{} `index:"cycle_start_at,status,id"`
	StatusCreatedAt        struct{} `index:"status,created_at,id"`
	CreatedAt              struct{} `index:"created_at"`
}

var userBenefitGrantUserRelation = orm.Relation{
	Field:      "user_id",
	Name:       "user",
	Option:     "user.NewUserModel",
	OptionKeys: []string{"name", "mobile", "status"},
}

var userBenefitGrantUserIdentityRelation = orm.Relation{
	Field:      "user_identity_id",
	Name:       "user_identity",
	Option:     "user.NewUserIdentityModel",
	OptionKeys: []string{"user_name", "user_mobile", "identity_name", "level_name", "expired_at", "status"},
}

var userBenefitGrantIdentityBenefitRelation = orm.Relation{
	Field:      "identity_benefit_id",
	Name:       "identity_benefit",
	Option:     "user.NewIdentityBenefitModel",
	OptionKeys: []string{"identity_name", "level_name", "point_name", "point_amount", "cycle_days", "limit_times", "clear_previous", "status"},
}

var userBenefitGrantPointRelation = orm.Relation{
	Field:      "point_config_id",
	Name:       "point_config",
	Option:     "user.NewPointConfigModel",
	OptionKeys: []string{"name", "symbol", "symbol_position"},
}

func NewUserBenefitGrantModel() *orm.Model[UserBenefitGrant] {
	return orm.LoadModel[UserBenefitGrant]("用户周期权益", "user_benefit_grant", orm.ModelConfig{
		Index:    UserBenefitGrantIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"status":                benefitGrantStatusOptions,
			"point_symbol_position": pointSymbolPositionOptions,
		},
		Relations: []orm.Relation{
			userBenefitGrantUserRelation,
			userBenefitGrantUserIdentityRelation,
			userBenefitGrantIdentityBenefitRelation,
			userBenefitGrantPointRelation,
		},
	})
}
