package api

import (
	"github.com/shemic/dever/server"

	userservice "github.com/dever-package/user/service"
)

type Auth struct{}

func (Auth) PostRegister(c *server.Context) error {
	body, err := bindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Register(c.Context(), userservice.RegisterRequest{
		Account:  bodyText(body, "account", "username", "mobile"),
		Password: bodyText(body, "password"),
		Name:     bodyText(body, "name", "nickname"),
	})
	return userJSON(c, data, err)
}

func (Auth) PostLogin(c *server.Context) error {
	body, err := bindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Login(c.Context(), userservice.LoginRequest{
		Account:  bodyText(body, "account", "username", "mobile"),
		Password: bodyText(body, "password"),
	})
	return userJSON(c, data, err)
}

func (Auth) PostRefresh(c *server.Context) error {
	body, err := bindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Refresh(c.Context(), bodyText(body, "refresh_token", "refreshToken"))
	return userJSON(c, data, err)
}

func (Auth) PostLogout(c *server.Context) error {
	body, err := bindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Logout(c.Context(), bodyText(body, "refresh_token", "refreshToken"))
	return userJSON(c, data, err)
}

func (Auth) GetProfile(c *server.Context) error {
	data, err := (userservice.AuthService{}).Profile(c.Context())
	return userJSON(c, data, err)
}
