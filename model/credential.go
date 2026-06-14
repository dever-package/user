package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

const (
	CredentialProviderPassword = "password"

	CredentialStatusEnabled  int16 = 1
	CredentialStatusDisabled int16 = 2
)

type Credential struct {
	ID           uint64    `dorm:"primaryKey;autoIncrement;comment:凭据ID"`
	UserID       uint64    `dorm:"type:bigint;not null;default:0;comment:用户"`
	Provider     string    `dorm:"type:varchar(32);not null;default:'password';comment:登录方式"`
	Account      string    `dorm:"type:varchar(128);not null;comment:账号"`
	PasswordHash string    `dorm:"type:varchar(255);not null;default:'';comment:密码哈希"`
	Status       int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	CreatedAt    time.Time `dorm:"comment:创建时间"`
}

type CredentialIndex struct {
	ProviderAccount struct{} `unique:"provider,account"`
	UserProvider    struct{} `index:"user_id,provider,status,id"`
	StatusCreatedAt struct{} `index:"status,created_at,id"`
}

func NewCredentialModel() *orm.Model[Credential] {
	return orm.LoadModel[Credential]("用户凭据", "user_credential", orm.ModelConfig{
		Index:    CredentialIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"provider": credentialProviderOptions,
			"status":   credentialStatusOptions,
		},
		Relations: []orm.Relation{
			userRelation,
		},
		Fields: map[string]orm.FieldConfig{
			"password_hash": {Type: orm.FieldTypePassword},
		},
	})
}
