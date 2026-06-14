package model

import "github.com/shemic/dever/orm"

var userRelation = orm.Relation{
	Field:      "user_id",
	Option:     "user.NewUserModel",
	OptionKeys: []string{"name", "mobile", "status"},
}
