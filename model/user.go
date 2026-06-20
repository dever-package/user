package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

const (
	UserStatusEnabled  int16 = 1
	UserStatusDisabled int16 = 2
)

type User struct {
	ID        uint64    `dorm:"primaryKey;autoIncrement;comment:用户ID"`
	Account   string    `dorm:"type:varchar(128);not null;default:'';comment:账号"`
	Name      string    `dorm:"type:varchar(64);not null;comment:姓名"`
	Mobile    string    `dorm:"type:varchar(32);not null;default:'';comment:手机号"`
	Status    int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	Remark    string    `dorm:"type:text;not null;default:'';comment:备注"`
	CreatedAt time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type UserIndex struct {
	Account   struct{} `index:"account"`
	Mobile    struct{} `index:"mobile"`
	Status    struct{} `index:"status,id"`
	CreatedAt struct{} `index:"created_at"`
}

func NewUserModel() *orm.Model[User] {
	return orm.LoadModel[User]("用户", "user_account", orm.ModelConfig{
		Index:    UserIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"status": userStatusOptions,
		},
	})
}
