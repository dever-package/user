package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type UserPoint struct {
	ID                  uint64    `dorm:"primaryKey;autoIncrement;comment:用户积分ID"`
	UserID              uint64    `dorm:"type:bigint;not null;default:0;comment:用户"`
	UserName            string    `dorm:"type:varchar(64);not null;default:'';comment:姓名"`
	UserMobile          string    `dorm:"type:varchar(32);not null;default:'';comment:手机号"`
	PointConfigID       uint64    `dorm:"type:bigint;not null;default:1;comment:积分"`
	PointName           string    `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string    `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16     `dorm:"type:smallint;not null;default:2;comment:符号位置"`
	Balance             int       `dorm:"type:int;not null;default:0;comment:积分余额"`
	TotalAdded          int       `dorm:"type:int;not null;default:0;comment:累计增加积分"`
	TotalUsed           int       `dorm:"type:int;not null;default:0;comment:累计消耗积分"`
	Version             int       `dorm:"type:int;not null;default:0;comment:版本号"`
	CreatedAt           time.Time `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type UserPointIndex struct {
	UserPoint            struct{} `unique:"user_id,point_config_id"`
	UserCreatedAt        struct{} `index:"user_id,created_at,id"`
	PointConfigCreatedAt struct{} `index:"point_config_id,created_at,id"`
	UserName             struct{} `index:"user_name,created_at,id"`
	UserMobile           struct{} `index:"user_mobile,created_at,id"`
	CreatedAt            struct{} `index:"created_at"`
}

var userPointUserRelation = orm.Relation{
	Field:      "user_id",
	Name:       "user",
	Option:     "user.NewUserModel",
	OptionKeys: []string{"name", "mobile", "status"},
}

var userPointConfigRelation = orm.Relation{
	Field:      "point_config_id",
	Name:       "point_config",
	Option:     "user.NewPointConfigModel",
	OptionKeys: []string{"name", "symbol", "symbol_position"},
}

func NewUserPointModel() *orm.Model[UserPoint] {
	return orm.LoadModel[UserPoint]("用户积分", "user_point", orm.ModelConfig{
		Index:    UserPointIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"point_symbol_position": pointSymbolPositionOptions,
		},
		Relations: []orm.Relation{
			userPointUserRelation,
			userPointConfigRelation,
		},
	})
}
