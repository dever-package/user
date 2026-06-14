package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type UserIdentityLog struct {
	ID             uint64    `dorm:"primaryKey;autoIncrement;comment:日志ID"`
	UserIdentityID uint64    `dorm:"type:bigint;not null;default:0;comment:用户身份"`
	UserID         uint64    `dorm:"type:bigint;not null;default:0;comment:用户"`
	UserName       string    `dorm:"type:varchar(64);not null;default:'';comment:姓名"`
	UserMobile     string    `dorm:"type:varchar(32);not null;default:'';comment:手机号"`
	IdentityID     uint64    `dorm:"type:bigint;not null;default:0;comment:身份"`
	IdentityName   string    `dorm:"type:varchar(64);not null;default:'';comment:身份名称"`
	LevelID        uint64    `dorm:"type:bigint;not null;default:0;comment:等级"`
	LevelName      string    `dorm:"type:varchar(64);not null;default:'';comment:等级名称"`
	Level          int       `dorm:"type:int;not null;default:1;comment:等级数字"`
	CardNo         string    `dorm:"type:varchar(32);not null;default:'';comment:会员卡号"`
	StartedAt      time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:开始时间"`
	ExpiredAt      time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:过期时间"`
	Remark         string    `dorm:"type:text;not null;default:'';comment:原因"`
	CreatedAt      time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type UserIdentityLogIndex struct {
	UserIdentityCreatedAt struct{} `index:"user_identity_id,created_at,id"`
	UserCreatedAt         struct{} `index:"user_id,created_at,id"`
	IdentityCreatedAt     struct{} `index:"identity_id,created_at,id"`
	LevelCreatedAt        struct{} `index:"level_id,created_at,id"`
	CardNo                struct{} `index:"card_no,created_at,id"`
	ExpiredAt             struct{} `index:"expired_at,id"`
	CreatedAt             struct{} `index:"created_at"`
}

func NewUserIdentityLogModel() *orm.Model[UserIdentityLog] {
	return orm.LoadModel[UserIdentityLog]("用户身份日志", "user_identity_log", orm.ModelConfig{
		Index:    UserIdentityLogIndex{},
		Order:    "id desc",
		Database: "default",
	})
}
