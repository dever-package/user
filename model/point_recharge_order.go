package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type PointRechargeOrder struct {
	ID                  uint64     `dorm:"primaryKey;autoIncrement;comment:订单ID"`
	OrderNo             string     `dorm:"type:varchar(40);not null;comment:订单号"`
	RequestID           string     `dorm:"type:varchar(64);not null;comment:请求幂等键"`
	UserID              uint64     `dorm:"type:bigint;not null;comment:用户"`
	UserName            string     `dorm:"type:varchar(64);not null;default:'';comment:用户名称"`
	UserMobile          string     `dorm:"type:varchar(32);not null;default:'';comment:用户手机号"`
	PackageID           uint64     `dorm:"type:bigint;not null;comment:积分套餐"`
	PackageName         string     `dorm:"type:varchar(64);not null;default:'';comment:套餐名称"`
	PointConfigID       uint64     `dorm:"type:bigint;not null;comment:积分类型"`
	PointName           string     `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string     `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16      `dorm:"type:smallint;not null;default:2;comment:积分符号位置"`
	PointAmount         int        `dorm:"type:int;not null;default:0;comment:基础积分"`
	BonusAmount         int        `dorm:"type:int;not null;default:0;comment:赠送积分"`
	TotalPoints         int        `dorm:"type:int;not null;default:0;comment:到账积分"`
	PayAmountMicros     int64      `dorm:"type:bigint;not null;default:0;comment:支付金额微单位"`
	Currency            string     `dorm:"type:varchar(8);not null;default:'CNY';comment:支付货币"`
	PaymentNo           string     `dorm:"type:varchar(128);not null;default:'';comment:支付单号"`
	PaymentURL          string     `dorm:"type:text;not null;default:'';comment:支付地址"`
	Status              string     `dorm:"type:varchar(24);not null;default:'pending_payment';comment:订单状态"`
	ErrorMessage        string     `dorm:"type:text;not null;default:'';comment:失败原因"`
	PaidAt              *time.Time `dorm:"null;comment:支付时间"`
	FulfilledAt         *time.Time `dorm:"null;comment:履约时间"`
	CreatedAt           time.Time  `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type PointRechargeOrderIndex struct {
	OrderNo       struct{} `unique:"order_no"`
	UserRequest   struct{} `unique:"user_id,request_id"`
	UserCreatedAt struct{} `index:"user_id,created_at,id"`
	UserStatus    struct{} `index:"user_id,status,id"`
	PaymentNo     struct{} `index:"payment_no"`
	Status        struct{} `index:"status,id"`
	CreatedAt     struct{} `index:"created_at"`
}

func NewPointRechargeOrderModel() *orm.Model[PointRechargeOrder] {
	return orm.LoadModel[PointRechargeOrder]("积分充值订单", "user_point_recharge_order", orm.ModelConfig{
		Index:    PointRechargeOrderIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"status": accountOrderStatusOptions,
		},
	})
}
