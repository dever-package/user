package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	feishuAPIBase        = "https://open.feishu.cn/open-apis"
	feishuRequestTimeout = 10 * time.Second
	feishuMaxResponse    = 1 << 20
)

type FeishuIdentity struct {
	OpenID          string `json:"open_id"`
	UnionID         string `json:"union_id"`
	UserID          string `json:"user_id"`
	Name            string `json:"name"`
	Mobile          string `json:"mobile"`
	Email           string `json:"email"`
	EnterpriseEmail string `json:"enterprise_email"`
	AvatarURL       string `json:"avatar_url"`
}

type feishuAppAccessTokenResponse struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	AppAccessToken string `json:"app_access_token"`
}

type feishuIdentityResponse struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data FeishuIdentity `json:"data"`
}

func ExchangeFeishuAuthorizationCode(ctx context.Context, appID string, appSecret string, code string) (FeishuIdentity, error) {
	appID = strings.TrimSpace(appID)
	appSecret = strings.TrimSpace(appSecret)
	code = strings.TrimSpace(code)
	if appID == "" || appSecret == "" {
		return FeishuIdentity{}, fmt.Errorf("飞书登录尚未完成配置")
	}
	if code == "" {
		return FeishuIdentity{}, fmt.Errorf("飞书授权码不能为空")
	}
	if len([]byte(appID)) > 128 || len([]byte(appSecret)) > 4096 || len([]byte(code)) > 4096 {
		return FeishuIdentity{}, fmt.Errorf("飞书登录参数长度不正确")
	}

	appAccessToken, err := fetchFeishuAppAccessToken(ctx, appID, appSecret)
	if err != nil {
		return FeishuIdentity{}, err
	}
	var response feishuIdentityResponse
	if err := postFeishuJSON(ctx, "/authen/v1/access_token", map[string]any{
		"grant_type": "authorization_code",
		"code":       code,
	}, appAccessToken, &response); err != nil {
		return FeishuIdentity{}, err
	}
	if response.Code != 0 {
		return FeishuIdentity{}, fmt.Errorf("飞书身份换取失败：%s", feishuResponseMessage(response.Msg))
	}
	response.Data.OpenID = strings.TrimSpace(response.Data.OpenID)
	if response.Data.OpenID == "" {
		return FeishuIdentity{}, fmt.Errorf("飞书未返回用户身份信息")
	}
	return response.Data, nil
}

func fetchFeishuAppAccessToken(ctx context.Context, appID string, appSecret string) (string, error) {
	var response feishuAppAccessTokenResponse
	if err := postFeishuJSON(ctx, "/auth/v3/app_access_token/internal", map[string]any{
		"app_id":     appID,
		"app_secret": appSecret,
	}, "", &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("飞书应用认证失败：%s", feishuResponseMessage(response.Msg))
	}
	token := strings.TrimSpace(response.AppAccessToken)
	if token == "" {
		return "", fmt.Errorf("飞书未返回应用访问令牌")
	}
	return token, nil
}

func postFeishuJSON(ctx context.Context, path string, body any, bearerToken string, target any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("构造飞书请求失败")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, feishuRequestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, feishuAPIBase+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("构造飞书请求失败")
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	if token := strings.TrimSpace(bearerToken); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("飞书接口请求失败：%w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, feishuMaxResponse+1))
	if err != nil {
		return fmt.Errorf("读取飞书响应失败")
	}
	if len(responseBody) > feishuMaxResponse {
		return fmt.Errorf("飞书响应内容过大")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("飞书接口请求失败（HTTP %d）", response.StatusCode)
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("解析飞书响应失败")
	}
	return nil
}

func feishuResponseMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "未知错误"
	}
	return truncateRunes(message, 160)
}
