package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type IdentityOrder struct {
	ID                  uint64     `dorm:"primaryKey;autoIncrement;comment:订单ID"`
	OrderNo             string     `dorm:"type:varchar(40);not null;comment:订单号"`
	RequestID           string     `dorm:"type:varchar(64);not null;comment:请求幂等键"`
	UserID              uint64     `dorm:"type:bigint;not null;comment:用户"`
	UserName            string     `dorm:"type:varchar(64);not null;default:'';comment:用户名称"`
	UserMobile          string     `dorm:"type:varchar(32);not null;default:'';comment:用户手机号"`
	IdentityID          uint64     `dorm:"type:bigint;not null;comment:身份"`
	IdentityName        string     `dorm:"type:varchar(64);not null;default:'';comment:身份名称"`
	LevelID             uint64     `dorm:"type:bigint;not null;comment:等级"`
	LevelName           string     `dorm:"type:varchar(64);not null;default:'';comment:等级名称"`
	Level               int        `dorm:"type:int;not null;default:0;comment:等级数字"`
	Action              string     `dorm:"type:varchar(24);not null;default:'subscribe';comment:订阅动作"`
	PointConfigID       uint64     `dorm:"type:bigint;not null;comment:支付积分"`
	PointName           string     `dorm:"type:varchar(64);not null;default:'';comment:积分名称"`
	PointSymbol         string     `dorm:"type:varchar(32);not null;default:'';comment:积分符号"`
	PointSymbolPosition int16      `dorm:"type:smallint;not null;default:2;comment:积分符号位置"`
	TotalPoints         int        `dorm:"type:int;not null;default:0;comment:订单积分"`
	BalancePoints       int        `dorm:"type:int;not null;default:0;comment:余额支付积分"`
	RechargePoints      int        `dorm:"type:int;not null;default:0;comment:充值缺口积分"`
	PayAmountMicros     int64      `dorm:"type:bigint;not null;default:0;comment:支付金额微单位"`
	Currency            string     `dorm:"type:varchar(8);not null;default:'CNY';comment:支付货币"`
	HoldBusinessKey     string     `dorm:"type:varchar(128);not null;default:'';comment:积分预占业务键"`
	PaymentNo           string     `dorm:"type:varchar(128);not null;default:'';comment:支付单号"`
	PaymentURL          string     `dorm:"type:text;not null;default:'';comment:支付地址"`
	Status              string     `dorm:"type:varchar(24);not null;default:'pending_payment';comment:订单状态"`
	ErrorMessage        string     `dorm:"type:text;not null;default:'';comment:失败原因"`
	PreviousExpiredAt   *time.Time `dorm:"null;comment:原到期时间"`
	TargetExpiredAt     *time.Time `dorm:"null;comment:目标到期时间"`
	PaidAt              *time.Time `dorm:"null;comment:支付时间"`
	FulfilledAt         *time.Time `dorm:"null;comment:履约时间"`
	CreatedAt           time.Time  `dorm:"not null;default:CURRENT_TIMESTAMP;comment:创建时间"`
}

type IdentityOrderIndex struct {
	OrderNo       struct{} `unique:"order_no"`
	UserRequest   struct{} `unique:"user_id,request_id"`
	UserCreatedAt struct{} `index:"user_id,created_at,id"`
	UserStatus    struct{} `index:"user_id,status,id"`
	PaymentNo     struct{} `index:"payment_no"`
	Status        struct{} `index:"status,id"`
	CreatedAt     struct{} `index:"created_at"`
}

func NewIdentityOrderModel() *orm.Model[IdentityOrder] {
	return orm.LoadModel[IdentityOrder]("身份订单", "user_identity_order", orm.ModelConfig{
		Index:    IdentityOrderIndex{},
		Order:    "id desc",
		Database: "default",
		Options: map[string]any{
			"status": accountOrderStatusOptions,
		},
	})
}
