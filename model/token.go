package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

const (
	TokenTypeRefresh = "refresh"

	TokenStatusEnabled int16 = 1
	TokenStatusRevoked int16 = 2
)

type Token struct {
	ID        uint64    `dorm:"primaryKey;autoIncrement;comment:Token ID"`
	UserID    uint64    `dorm:"type:bigint;not null;default:0;comment:用户"`
	Type      string    `dorm:"type:varchar(32);not null;default:'refresh';comment:类型"`
	TokenHash string    `dorm:"type:varchar(128);not null;comment:Token哈希"`
	Status    int16     `dorm:"type:smallint;not null;default:1;comment:状态"`
	ExpiresAt time.Time `dorm:"comment:过期时间"`
	CreatedAt time.Time `dorm:"comment:创建时间"`
	UsedAt    time.Time `dorm:"comment:最后使用时间"`
}

type TokenIndex struct {
	TokenHash       struct{} `unique:"token_hash"`
	UserTypeStatus  struct{} `index:"user_id,type,status,expires_at,id"`
	StatusExpiresAt struct{} `index:"status,expires_at,id"`
}

func NewTokenModel() *orm.Model[Token] {
	return orm.LoadModel[Token]("用户Token", "user_token", orm.ModelConfig{
		Index:    TokenIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"type": []map[string]any{
				{"id": TokenTypeRefresh, "value": "刷新Token", "label": "刷新Token", "color": "#2563eb"},
			},
			"status": tokenStatusOptions,
		},
		Relations: []orm.Relation{
			userRelation,
		},
		Fields: map[string]orm.FieldConfig{
			"token_hash": {Type: orm.FieldTypePassword},
		},
	})
}
