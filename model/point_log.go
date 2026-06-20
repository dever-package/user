package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type PointLog struct {
	ID                  uint64    `dorm:"primaryKey;autoIncrement;comment:日志ID"`
	UserPointID         uint64    `dorm:"type:bigint;not null;default:0;comment:用户积分"`
	UserID              uint64    `dorm:"type:bigint;not null;comment:用户"`
	UserName            string    `dorm:"type:varchar(64);not null;default:'';comment:姓名"`
	UserMobile          string    `dorm:"type:varchar(32);not null;default:'';comment:手机号"`
	PointConfigID       uint64    `dorm:"type:bigint;not null;default:1;comment:积分"`
	PointName           string    `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string    `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16     `dorm:"type:smallint;not null;default:2;comment:符号位置"`
	ChangeType          string    `dorm:"type:varchar(32);not null;comment:变动类型"`
	Source              string    `dorm:"type:varchar(32);not null;default:'admin';comment:来源"`
	Amount              int       `dorm:"type:int;not null;comment:变动积分"`
	BalanceBefore       int       `dorm:"type:int;not null;default:0;comment:变动前余额"`
	BalanceAfter        int       `dorm:"type:int;not null;default:0;comment:变动后余额"`
	Remark              string    `dorm:"type:text;not null;default:'';comment:备注"`
	CreatedAt           time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type PointLogIndex struct {
	UserPointCreatedAt   struct{} `index:"user_point_id,created_at,id"`
	UserCreatedAt        struct{} `index:"user_id,created_at,id"`
	PointConfigCreatedAt struct{} `index:"point_config_id,created_at,id"`
	UserName             struct{} `index:"user_name,created_at,id"`
	UserMobile           struct{} `index:"user_mobile,created_at,id"`
	ChangeType           struct{} `index:"change_type,created_at,id"`
	Source               struct{} `index:"source,created_at,id"`
	CreatedAt            struct{} `index:"created_at"`
}

var pointLogUserRelation = orm.Relation{
	Field:      "user_id",
	Name:       "user",
	Option:     "user.NewUserModel",
	OptionKeys: []string{"name", "mobile"},
}

var pointLogUserPointRelation = orm.Relation{
	Field:      "user_point_id",
	Name:       "user_point",
	Option:     "user.NewUserPointModel",
	OptionKeys: []string{"user_name", "user_mobile", "point_name", "balance"},
}

var pointLogConfigRelation = orm.Relation{
	Field:      "point_config_id",
	Name:       "point_config",
	Option:     "user.NewPointConfigModel",
	OptionKeys: []string{"name", "symbol", "symbol_position"},
}

func NewPointLogModel() *orm.Model[PointLog] {
	return orm.LoadModel[PointLog]("积分日志", "user_point_log", orm.ModelConfig{
		Index:    PointLogIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"change_type": pointChangeTypeOptions,
			"source":      pointSourceOptions,
		},
		Relations: []orm.Relation{
			pointLogUserPointRelation,
			pointLogUserRelation,
			pointLogConfigRelation,
		},
	})
}
