package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

const (
	PointHoldStatusActive   = 1
	PointHoldStatusSettled  = 2
	PointHoldStatusReleased = 3
	PointHoldStatusExpired  = 4
)

type PointHold struct {
	ID                  uint64     `dorm:"primaryKey;autoIncrement;comment:积分预占ID"`
	BusinessKey         string     `dorm:"type:varchar(128);not null;comment:业务幂等键"`
	BusinessType        string     `dorm:"type:varchar(32);not null;default:'ability';comment:业务类型"`
	UserPointID         uint64     `dorm:"type:bigint;not null;default:0;comment:用户积分"`
	UserID              uint64     `dorm:"type:bigint;not null;default:0;comment:用户"`
	UserName            string     `dorm:"type:varchar(64);not null;default:'';comment:姓名"`
	UserMobile          string     `dorm:"type:varchar(32);not null;default:'';comment:手机号"`
	PointConfigID       uint64     `dorm:"type:bigint;not null;default:1;comment:积分"`
	PointName           string     `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string     `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16      `dorm:"type:smallint;not null;default:2;comment:符号位置"`
	ReservedAmount      int        `dorm:"type:int;not null;default:0;comment:预占积分"`
	SettledAmount       int        `dorm:"type:int;not null;default:0;comment:实扣积分"`
	Status              int16      `dorm:"type:smallint;not null;default:1;comment:状态"`
	Remark              string     `dorm:"type:text;not null;default:'';comment:备注"`
	ExpiresAt           time.Time  `dorm:"not null;comment:过期时间"`
	SettledAt           *time.Time `dorm:"null;comment:结算时间"`
	ReleasedAt          *time.Time `dorm:"null;comment:释放时间"`
	CreatedAt           time.Time  `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type PointHoldIndex struct {
	BusinessKey       struct{} `unique:"business_key"`
	UserPointStatus   struct{} `index:"user_point_id,status,expires_at,id"`
	UserStatus        struct{} `index:"user_id,status,created_at,id"`
	PointStatus       struct{} `index:"point_config_id,status,created_at,id"`
	BusinessCreatedAt struct{} `index:"business_type,created_at,id"`
	ExpiresStatus     struct{} `index:"expires_at,status,id"`
	CreatedAt         struct{} `index:"created_at"`
}

var (
	pointHoldUserRelation = orm.Relation{
		Field:      "user_id",
		Name:       "user",
		Option:     "user.NewUserModel",
		OptionKeys: []string{"name", "mobile", "status"},
	}

	pointHoldUserPointRelation = orm.Relation{
		Field:      "user_point_id",
		Name:       "user_point",
		Option:     "user.NewUserPointModel",
		OptionKeys: []string{"balance", "version"},
	}

	pointHoldPointRelation = orm.Relation{
		Field:      "point_config_id",
		Name:       "point_config",
		Option:     "user.NewPointConfigModel",
		OptionKeys: []string{"name", "exchange_rate", "symbol", "symbol_position"},
	}
)

func NewPointHoldModel() *orm.Model[PointHold] {
	return orm.LoadModel[PointHold]("积分预占", "user_point_hold", orm.ModelConfig{
		Index:    PointHoldIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"status":                pointHoldStatusOptions,
			"point_symbol_position": pointSymbolPositionOptions,
		},
		Relations: []orm.Relation{
			pointHoldUserRelation,
			pointHoldUserPointRelation,
			pointHoldPointRelation,
		},
	})
}
