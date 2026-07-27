package api

import (
	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	userapi "github.com/dever-package/user/api"
	userservice "github.com/dever-package/user/service"
)

type User struct{}

func (User) PostSave(c *server.Context) error {
	body, err := userapi.BindBody(c)
	if err != nil {
		return c.Error(err)
	}
	request := userservice.AdminUserSaveRequest{
		ID:       util.ToUint64(body["id"]),
		Account:  userapi.BodyText(body, "account"),
		Name:     userapi.BodyText(body, "name"),
		Password: userapi.BodyText(body, "password"),
		Status:   int16(util.ToIntDefault(body["status"], 0)),
		Remark:   userapi.BodyText(body, "remark"),
	}
	data, err := (userservice.AdminUserService{}).Save(c.Context(), request)
	return userapi.WriteJSON(c, data, err)
}
