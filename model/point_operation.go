package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

const (
	PointOperationStatusProcessing = "processing"
	PointOperationStatusCompleted  = "completed"
)

type PointOperation struct {
	ID            uint64    `dorm:"primaryKey;autoIncrement;comment:积分操作ID"`
	BusinessKey   string    `dorm:"type:varchar(128);not null;comment:业务幂等键"`
	UserPointID   uint64    `dorm:"type:bigint;not null;default:0;comment:用户积分"`
	UserID        uint64    `dorm:"type:bigint;not null;default:0;comment:用户"`
	PointConfigID uint64    `dorm:"type:bigint;not null;default:1;comment:积分"`
	ChangeType    string    `dorm:"type:varchar(32);not null;comment:变动类型"`
	Source        string    `dorm:"type:varchar(32);not null;default:'admin';comment:来源"`
	Amount        int       `dorm:"type:int;not null;comment:变动积分"`
	Remark        string    `dorm:"type:text;not null;default:'';comment:备注"`
	Status        string    `dorm:"type:varchar(16);not null;default:'processing';comment:操作状态"`
	CreatedAt     time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type PointOperationIndex struct {
	BusinessKey struct{} `unique:"business_key"`
	UserCreated struct{} `index:"user_id,created_at,id"`
	Status      struct{} `index:"status,created_at,id"`
	CreatedAt   struct{} `index:"created_at"`
}

func NewPointOperationModel() *orm.Model[PointOperation] {
	return orm.LoadModel[PointOperation]("积分操作", "user_point_operation", orm.ModelConfig{
		Index:    PointOperationIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"change_type": pointChangeTypeOptions,
			"source":      pointSourceOptions,
			"status": []map[string]any{
				{"id": PointOperationStatusProcessing, "value": "处理中"},
				{"id": PointOperationStatusCompleted, "value": "已完成"},
			},
		},
	})
}
