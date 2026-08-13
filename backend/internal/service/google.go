package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const googleOAuthSettingKey = "google_oauth"

type GoogleOAuthSettingRequest struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectUrl"`
}
type PublicGoogleOAuthSetting struct {
	Enabled         bool      `json:"enabled"`
	ClientID        string    `json:"clientId"`
	HasClientSecret bool      `json:"hasClientSecret"`
	RedirectURL     string    `json:"redirectUrl"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
type googleOAuthSettingValue struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectUrl"`
}
type googleProfile struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (s *Service) AdminGoogleOAuthSetting(actor *model.User) (*PublicGoogleOAuthSetting, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	setting, value, err := s.readGoogleOAuthSetting()
	if err != nil {
		return nil, err
	}
	result := publicGoogleOAuthSetting(value)
	if setting != nil {
		result.UpdatedAt = setting.UpdatedAt
	}
	return &result, nil
}
func (s *Service) UpdateGoogleOAuthSetting(actor *model.User, req GoogleOAuthSettingRequest) (*PublicGoogleOAuthSetting, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	currentSetting, current, err := s.readGoogleOAuthSetting()
	if err != nil {
		return nil, err
	}
	next := googleOAuthSettingValue{Enabled: req.Enabled, ClientID: strings.TrimSpace(req.ClientID), ClientSecret: strings.TrimSpace(req.ClientSecret), RedirectURL: strings.TrimSpace(req.RedirectURL)}
	if next.ClientSecret == "" {
		next.ClientSecret = current.ClientSecret
	}
	if next.Enabled {
		if next.ClientID == "" || next.ClientSecret == "" || next.RedirectURL == "" {
			return nil, BadAuthRequest("启用 Google 登录前请完整填写 Client ID、Client Secret 和回调地址")
		}
		u, e := url.Parse(next.RedirectURL)
		if e != nil || u.Scheme == "" || u.Host == "" {
			return nil, BadAuthRequest("Google 回调地址无效")
		}
	}
	stored := next
	stored.ClientSecret, err = s.encryptSettingSecret(next.ClientSecret)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	setting := model.SystemSetting{Key: googleOAuthSettingKey, ValueJSON: string(raw), UpdatedBy: actor.ID}
	if currentSetting != nil {
		setting.CreatedAt = currentSetting.CreatedAt
	}
	if err = s.repo.SaveSystemSetting(&setting); err != nil {
		return nil, err
	}
	result := publicGoogleOAuthSetting(next)
	result.UpdatedAt = setting.UpdatedAt
	return &result, nil
}
func (s *Service) GoogleOAuthEnabled() bool {
	_, v, e := s.readGoogleOAuthSetting()
	return e == nil && v.Enabled
}
func (s *Service) BeginGoogleLogin(nextPath string) (string, error) {
	count, err := s.repo.UserCount()
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", Forbidden("请先创建本地管理员账号，再开放 Google 登录")
	}
	_, setting, err := s.readGoogleOAuthSetting()
	if err != nil {
		return "", err
	}
	if !setting.Enabled {
		return "", Forbidden("Google 登录尚未启用")
	}
	state := randomToken()
	if err = s.repo.CreateOAuthState(&model.OAuthState{ID: newID(), Provider: "google", StateHash: hashToken(state), NextPath: safeOAuthNext(nextPath), ExpiresAt: time.Now().Add(10 * time.Minute)}); err != nil {
		return "", err
	}
	q := url.Values{"client_id": {setting.ClientID}, "redirect_uri": {setting.RedirectURL}, "response_type": {"code"}, "scope": {"openid email profile"}, "state": {state}, "access_type": {"online"}, "prompt": {"select_account"}}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode(), nil
}
func (s *Service) CompleteGoogleLogin(stateValue, code string) (*LinuxDOCallbackResult, error) {
	if strings.TrimSpace(stateValue) == "" || strings.TrimSpace(code) == "" {
		return nil, BadAuthRequest("Google 登录回调缺少必要参数")
	}
	state, err := s.repo.ConsumeOAuthState("google", hashToken(stateValue))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, BadAuthRequest("Google 登录状态无效或已过期")
	}
	if err != nil {
		return nil, err
	}
	_, setting, err := s.readGoogleOAuthSetting()
	if err != nil {
		return nil, err
	}
	token, err := exchangeGoogleCode(setting, code)
	if err != nil {
		return nil, err
	}
	profile, err := fetchGoogleProfile(token)
	if err != nil {
		return nil, err
	}
	if profile.Subject == "" || profile.Email == "" || !profile.EmailVerified {
		return nil, Forbidden("Google 账号必须具有已验证邮箱")
	}
	identity, err := s.repo.UserIdentity("google", profile.Subject)
	var user *model.User
	if err == nil {
		user, err = s.repo.User(identity.UserID)
		if err != nil {
			return nil, err
		}
		identity.ProviderUsername = profile.Email
		identity.AvatarURL = profile.Picture
		identity.UpdatedAt = time.Now()
		if err = s.repo.Save(identity); err != nil {
			return nil, err
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		enabled, e := s.RegistrationEnabled()
		if e != nil {
			return nil, e
		}
		if !enabled {
			return nil, Forbidden("管理员未开放新用户注册")
		}
		inviteCode := inviteCodeFromNext(state.NextPath)
		if inviteCode != "" {
			if _, _, e = s.validateInviteCode("", inviteCode); e != nil {
				return nil, e
			}
		}
		user, identity, err = s.createGoogleUser(profile)
		if err != nil {
			return nil, err
		}
		if err = s.repo.CreateOAuthUser(user, identity); err != nil {
			return nil, err
		}
		if err = s.ensureSignupBonus(user.ID); err != nil {
			return nil, err
		}
		if inviteCode != "" {
			if err = s.applyInviteCode(user.ID, inviteCode); err != nil {
				return nil, err
			}
		}
	} else {
		return nil, err
	}
	if user.Status != model.UserStatusActive {
		return nil, Forbidden("该账号已被禁用")
	}
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err = s.repo.Save(user); err != nil {
		return nil, err
	}
	s.recordActivity(user.ID, "login", 1)
	session, err := s.createAuthSession(user)
	if err != nil {
		return nil, err
	}
	return &LinuxDOCallbackResult{Session: session, Next: safeOAuthNext(state.NextPath)}, nil
}
func (s *Service) createGoogleUser(p googleProfile) (*model.User, *model.UserIdentity, error) {
	base := strings.Split(p.Email, "@")[0]
	base = oauthUsernameSanitizer.ReplaceAllString(base, "_")
	if len(base) < 3 {
		base = "google_" + shortSubject(p.Subject)
	}
	if len(base) > 24 {
		base = base[:24]
	}
	username := base
	if existing, e := s.repo.UserByUsername(username); e == nil && existing != nil {
		username = truncateRunes(base, 23) + "_" + shortSubject(p.Subject)
	}
	email := normalizeEmail(p.Email)
	if existing, e := s.repo.UserByEmail(email); e == nil && existing != nil {
		return nil, nil, BadAuthRequest("该 Google 邮箱已绑定其他账号，请使用原登录方式")
	}
	user := &model.User{ID: newID(), Username: username, Email: email, DisplayName: normalizeDisplayName(p.Name, username), Role: model.UserRoleUser, Status: model.UserStatusActive}
	identity := &model.UserIdentity{ID: newID(), UserID: user.ID, Provider: "google", Subject: p.Subject, ProviderUsername: p.Email, AvatarURL: p.Picture}
	return user, identity, nil
}
func (s *Service) readGoogleOAuthSetting() (*model.SystemSetting, googleOAuthSettingValue, error) {
	setting, err := s.repo.SystemSetting(googleOAuthSettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, googleOAuthSettingValue{}, nil
	}
	if err != nil {
		return nil, googleOAuthSettingValue{}, err
	}
	var value googleOAuthSettingValue
	if json.Unmarshal([]byte(setting.ValueJSON), &value) != nil {
		return nil, value, errors.New("Google OAuth 配置格式无效")
	}
	value.ClientSecret, err = s.decryptSettingSecret(value.ClientSecret)
	return setting, value, err
}
func publicGoogleOAuthSetting(v googleOAuthSettingValue) PublicGoogleOAuthSetting {
	return PublicGoogleOAuthSetting{Enabled: v.Enabled, ClientID: v.ClientID, HasClientSecret: v.ClientSecret != "", RedirectURL: v.RedirectURL}
}
func exchangeGoogleCode(s googleOAuthSettingValue, code string) (string, error) {
	form := url.Values{"client_id": {s.ClientID}, "client_secret": {s.ClientSecret}, "code": {code}, "grant_type": {"authorization_code"}, "redirect_uri": {s.RedirectURL}}
	req, _ := http.NewRequest(http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := OutboundHTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Google 授权失败：%s", resp.Status)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.AccessToken == "" {
		return "", errors.New("Google 未返回访问令牌")
	}
	return payload.AccessToken, nil
}
func fetchGoogleProfile(token string) (googleProfile, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := OutboundHTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return googleProfile{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return googleProfile{}, fmt.Errorf("读取 Google 用户信息失败：%s", resp.Status)
	}
	var p googleProfile
	err = json.Unmarshal(body, &p)
	return p, err
}
func inviteCodeFromNext(next string) string {
	u, e := url.Parse(next)
	if e != nil {
		return ""
	}
	return u.Query().Get("invite")
}
