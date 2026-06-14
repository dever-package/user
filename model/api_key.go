package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

const (
	APIKeyStatusEnabled  int16 = 1
	APIKeyStatusDisabled int16 = 2
)

type APIKey struct {
	ID         uint64    `dorm:"primaryKey;autoIncrement;comment:API Key ID"`
	UserID     uint64    `dorm:"type:bigint;not null;default:0;comment:用户"`
	Name       string    `dorm:"type:varchar(128);not null;comment:名称"`
	Prefix     string    `dorm:"type:varchar(32);not null;comment:前缀"`
	KeyHash    string    `dorm:"type:varchar(128);not null;comment:密钥哈希"`
	Scopes     string    `dorm:"type:text;not null;default:'[]';comment:权限范围"`
	Status     int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	ExpiresAt  time.Time `dorm:"comment:过期时间"`
	LastUsedAt time.Time `dorm:"comment:最后使用时间"`
	CreatedAt  time.Time `dorm:"comment:创建时间"`
}

type APIKeyIndex struct {
	KeyHash       struct{} `unique:"key_hash"`
	UserStatus    struct{} `index:"user_id,status,id"`
	PrefixStatus  struct{} `index:"prefix,status,id"`
	StatusExpires struct{} `index:"status,expires_at,id"`
}

func NewAPIKeyModel() *orm.Model[APIKey] {
	return orm.LoadModel[APIKey]("用户API Key", "user_api_key", orm.ModelConfig{
		Index:    APIKeyIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"status": apiKeyStatusOptions,
		},
		Relations: []orm.Relation{
			userRelation,
		},
		Fields: map[string]orm.FieldConfig{
			"key_hash": {Type: orm.FieldTypePassword},
		},
	})
}
