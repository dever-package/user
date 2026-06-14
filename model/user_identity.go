package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type UserIdentity struct {
	ID           uint64    `dorm:"primaryKey;autoIncrement;comment:用户身份ID"`
	UserID       uint64    `dorm:"type:bigint;not null;default:0;comment:用户"`
	UserName     string    `dorm:"type:varchar(64);not null;default:'';comment:姓名"`
	UserMobile   string    `dorm:"type:varchar(32);not null;default:'';comment:手机号"`
	IdentityID   uint64    `dorm:"type:bigint;not null;default:0;comment:身份"`
	IdentityName string    `dorm:"type:varchar(64);not null;default:'';comment:身份名称"`
	LevelID      uint64    `dorm:"type:bigint;not null;default:0;comment:等级"`
	LevelName    string    `dorm:"type:varchar(64);not null;default:'';comment:等级名称"`
	Level        int       `dorm:"type:int;not null;default:1;comment:等级数字"`
	CardNo       string    `dorm:"type:varchar(32);not null;default:'';comment:会员卡号"`
	ExpiredAt    time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:过期时间"`
	Status       int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	CreatedAt    time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type UserIdentityIndex struct {
	UserIdentity      struct{} `unique:"user_id,identity_id"`
	UserCreatedAt     struct{} `index:"user_id,created_at,id"`
	IdentityCreatedAt struct{} `index:"identity_id,created_at,id"`
	LevelCreatedAt    struct{} `index:"level_id,created_at,id"`
	CardNo            struct{} `index:"card_no"`
	ExpiredAt         struct{} `index:"expired_at,id"`
	UserName          struct{} `index:"user_name,created_at,id"`
	UserMobile        struct{} `index:"user_mobile,created_at,id"`
	Status            struct{} `index:"status,id"`
	CreatedAt         struct{} `index:"created_at"`
}

var userIdentityUserRelation = orm.Relation{
	Field:      "user_id",
	Name:       "user",
	Option:     "user.NewUserModel",
	OptionKeys: []string{"name", "mobile", "status"},
}

var userIdentityIdentityRelation = orm.Relation{
	Field:      "identity_id",
	Name:       "identity",
	Option:     "user.NewIdentityModel",
	OptionKeys: []string{"name", "status"},
}

var userIdentityLevelRelation = orm.Relation{
	Field:      "level_id",
	Name:       "identity_level",
	Option:     "user.NewIdentityLevelModel",
	OptionKeys: []string{"identity_id", "name", "level", "status"},
}

func NewUserIdentityModel() *orm.Model[UserIdentity] {
	return orm.LoadModel[UserIdentity]("用户身份", "user_identity", orm.ModelConfig{
		Index:    UserIdentityIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"status": identityStatusOptions,
		},
		Relations: []orm.Relation{
			userIdentityUserRelation,
			userIdentityIdentityRelation,
			userIdentityLevelRelation,
		},
	})
}
