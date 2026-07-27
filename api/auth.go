package api

import (
	"github.com/shemic/dever/server"

	userservice "github.com/dever-package/user/service"
)

type Auth struct{}

func (Auth) PostRegister(c *server.Context) error {
	body, err := BindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Register(c.Context(), userservice.RegisterRequest{
		Account:  BodyText(body, "account", "username", "mobile"),
		Password: BodyText(body, "password"),
		Name:     BodyText(body, "name", "nickname"),
	})
	return userJSON(c, data, err)
}

func (Auth) PostLogin(c *server.Context) error {
	body, err := BindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Login(c.Context(), userservice.LoginRequest{
		Account:  BodyText(body, "account", "username", "mobile"),
		Password: BodyText(body, "password"),
	})
	return userJSON(c, data, err)
}

func (Auth) PostRefresh(c *server.Context) error {
	body, err := BindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Refresh(c.Context(), BodyText(body, "refresh_token", "refreshToken"))
	return userJSON(c, data, err)
}

func (Auth) PostLogout(c *server.Context) error {
	body, err := BindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.AuthService{}).Logout(c.Context(), BodyText(body, "refresh_token", "refreshToken"))
	return userJSON(c, data, err)
}

func (Auth) GetProfile(c *server.Context) error {
	data, err := (userservice.AuthService{}).Profile(c.Context())
	return userJSON(c, data, err)
}

func (Auth) PostProfile(c *server.Context) error {
	body, err := BindBody(c)
	if err != nil {
		return c.Error(err)
	}
	avatarFileID, avatarFileIDSet := bodyOptionalUint64(body, "avatar_file_id", "avatarFileId")
	data, err := (userservice.ProfileService{}).Update(c.Context(), userservice.UpdateProfileRequest{
		Name:            BodyText(body, "name", "nickname"),
		AvatarFileID:    avatarFileID,
		AvatarFileIDSet: avatarFileIDSet,
	})
	return userJSON(c, data, err)
}

func (Auth) PostPassword(c *server.Context) error {
	body, err := BindBody(c)
	if err != nil {
		return c.Error(err)
	}
	data, err := (userservice.ProfileService{}).ChangePassword(c.Context(), userservice.ChangePasswordRequest{
		CurrentPassword: BodyText(body, "current_password", "currentPassword"),
		NewPassword:     BodyText(body, "new_password", "newPassword"),
	})
	return userJSON(c, data, err)
}
