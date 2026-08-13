package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const paymentSettingKey = "payments"

type PaymentPackage struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	CreditMicrocredits int64  `json:"creditMicrocredits"`
	CNYAmountMinor     int64  `json:"cnyAmountMinor"`
	USDAmountMinor     int64  `json:"usdAmountMinor"`
}

type AlipayPaymentSetting struct {
	Enabled               bool   `json:"enabled"`
	Sandbox               bool   `json:"sandbox"`
	AppID                 string `json:"appId"`
	SellerID              string `json:"sellerId"`
	MerchantPrivateKey    string `json:"merchantPrivateKey,omitempty"`
	AlipayPublicKey       string `json:"alipayPublicKey"`
	HasMerchantPrivateKey bool   `json:"hasMerchantPrivateKey,omitempty"`
}

type WeChatPaymentSetting struct {
	Enabled               bool   `json:"enabled"`
	AppID                 string `json:"appId"`
	MerchantID            string `json:"merchantId"`
	MerchantSerialNo      string `json:"merchantSerialNo"`
	MerchantPrivateKey    string `json:"merchantPrivateKey,omitempty"`
	APIV3Key              string `json:"apiV3Key,omitempty"`
	PlatformPublicKeyID   string `json:"platformPublicKeyId"`
	PlatformPublicKey     string `json:"platformPublicKey"`
	HasMerchantPrivateKey bool   `json:"hasMerchantPrivateKey,omitempty"`
	HasAPIV3Key           bool   `json:"hasApiV3Key,omitempty"`
}

type StripePaymentSetting struct {
	Enabled          bool   `json:"enabled"`
	SecretKey        string `json:"secretKey,omitempty"`
	WebhookSecret    string `json:"webhookSecret,omitempty"`
	HasSecretKey     bool   `json:"hasSecretKey,omitempty"`
	HasWebhookSecret bool   `json:"hasWebhookSecret,omitempty"`
}

type PayPalPaymentSetting struct {
	Enabled         bool   `json:"enabled"`
	Sandbox         bool   `json:"sandbox"`
	ClientID        string `json:"clientId"`
	ClientSecret    string `json:"clientSecret,omitempty"`
	WebhookID       string `json:"webhookId"`
	HasClientSecret bool   `json:"hasClientSecret,omitempty"`
}

type GenericPaymentSetting struct {
	Enabled          bool   `json:"enabled"`
	Name             string `json:"name"`
	Currency         string `json:"currency"`
	CreateURL        string `json:"createUrl"`
	APIKeyHeader     string `json:"apiKeyHeader"`
	APIKey           string `json:"apiKey,omitempty"`
	WebhookSecret    string `json:"webhookSecret,omitempty"`
	HasAPIKey        bool   `json:"hasApiKey,omitempty"`
	HasWebhookSecret bool   `json:"hasWebhookSecret,omitempty"`
}

type PaymentSetting struct {
	Enabled   bool                  `json:"enabled"`
	BaseURL   string                `json:"baseUrl"`
	Packages  []PaymentPackage      `json:"packages"`
	Alipay    AlipayPaymentSetting  `json:"alipay"`
	WeChat    WeChatPaymentSetting  `json:"wechat"`
	Stripe    StripePaymentSetting  `json:"stripe"`
	PayPal    PayPalPaymentSetting  `json:"paypal"`
	Generic   GenericPaymentSetting `json:"generic"`
	UpdatedBy string                `json:"updatedBy,omitempty"`
	UpdatedAt *time.Time            `json:"updatedAt,omitempty"`
}

func (s *Service) AdminPaymentSetting(actor *model.User) (*PaymentSetting, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	setting, value, err := s.readPaymentSetting()
	if err != nil {
		return nil, err
	}
	return publicPaymentSetting(setting, value), nil
}

func (s *Service) PublicPaymentSetting(actor *model.User) (*PaymentSetting, error) {
	if actor == nil {
		return nil, Unauthorized("请先登录")
	}
	_, value, err := s.readPaymentSetting()
	if err != nil {
		return nil, err
	}
	return publicPaymentSetting(nil, value), nil
}

func (s *Service) UpdatePaymentSetting(actor *model.User, req PaymentSetting) (*PaymentSetting, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	currentSetting, current, err := s.readPaymentSetting()
	if err != nil {
		return nil, err
	}
	next, err := normalizePaymentSetting(req, current)
	if err != nil {
		return nil, err
	}
	if err := validatePaymentSetting(next); err != nil {
		return nil, err
	}
	stored := next
	for _, secret := range []struct {
		target *string
		value  string
	}{
		{&stored.Alipay.MerchantPrivateKey, next.Alipay.MerchantPrivateKey},
		{&stored.WeChat.MerchantPrivateKey, next.WeChat.MerchantPrivateKey},
		{&stored.WeChat.APIV3Key, next.WeChat.APIV3Key},
		{&stored.Stripe.SecretKey, next.Stripe.SecretKey},
		{&stored.Stripe.WebhookSecret, next.Stripe.WebhookSecret},
		{&stored.PayPal.ClientSecret, next.PayPal.ClientSecret},
		{&stored.Generic.APIKey, next.Generic.APIKey},
		{&stored.Generic.WebhookSecret, next.Generic.WebhookSecret},
	} {
		*secret.target, err = s.encryptSettingSecret(secret.value)
		if err != nil {
			return nil, err
		}
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	record := &model.SystemSetting{Key: paymentSettingKey, ValueJSON: string(encoded), UpdatedBy: actor.ID}
	if currentSetting != nil {
		record.CreatedAt = currentSetting.CreatedAt
	}
	if err := s.repo.SaveSystemSetting(record); err != nil {
		return nil, err
	}
	_ = s.appendAdminAudit(actor, "payments.settings.update", "system_setting", paymentSettingKey, "更新支付渠道配置", map[string]any{"enabled": next.Enabled})
	return publicPaymentSetting(record, next), nil
}

func (s *Service) readPaymentSetting() (*model.SystemSetting, PaymentSetting, error) {
	record, err := s.repo.SystemSetting(paymentSettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, defaultPaymentSetting(), nil
	}
	if err != nil {
		return nil, PaymentSetting{}, err
	}
	value := defaultPaymentSetting()
	if err := json.Unmarshal([]byte(record.ValueJSON), &value); err != nil {
		return nil, PaymentSetting{}, errors.New("支付配置格式无效")
	}
	for _, secret := range []*string{
		&value.Alipay.MerchantPrivateKey, &value.WeChat.MerchantPrivateKey, &value.WeChat.APIV3Key,
		&value.Stripe.SecretKey, &value.Stripe.WebhookSecret, &value.PayPal.ClientSecret,
		&value.Generic.APIKey, &value.Generic.WebhookSecret,
	} {
		*secret, err = s.decryptSettingSecret(*secret)
		if err != nil {
			return nil, PaymentSetting{}, err
		}
	}
	return record, value, nil
}

func defaultPaymentSetting() PaymentSetting {
	return PaymentSetting{
		Packages: []PaymentPackage{},
		Generic:  GenericPaymentSetting{Name: "其他国际支付", Currency: "USD", APIKeyHeader: "Authorization"},
	}
}

func normalizePaymentSetting(req PaymentSetting, current PaymentSetting) (PaymentSetting, error) {
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if req.BaseURL != "" {
		parsed, err := url.Parse(req.BaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return PaymentSetting{}, BadAuthRequest("支付回调基础地址必须是完整的 HTTP 或 HTTPS 地址")
		}
	}
	keepSecret(&req.Alipay.MerchantPrivateKey, current.Alipay.MerchantPrivateKey)
	keepSecret(&req.WeChat.MerchantPrivateKey, current.WeChat.MerchantPrivateKey)
	keepSecret(&req.WeChat.APIV3Key, current.WeChat.APIV3Key)
	keepSecret(&req.Stripe.SecretKey, current.Stripe.SecretKey)
	keepSecret(&req.Stripe.WebhookSecret, current.Stripe.WebhookSecret)
	keepSecret(&req.PayPal.ClientSecret, current.PayPal.ClientSecret)
	keepSecret(&req.Generic.APIKey, current.Generic.APIKey)
	keepSecret(&req.Generic.WebhookSecret, current.Generic.WebhookSecret)
	seen := map[string]bool{}
	for index := range req.Packages {
		item := &req.Packages[index]
		item.ID = strings.TrimSpace(item.ID)
		item.Name = truncateRunes(strings.TrimSpace(item.Name), 80)
		if item.ID == "" || item.Name == "" || seen[item.ID] {
			return PaymentSetting{}, BadAuthRequest("充值套餐 ID 和名称不能为空且 ID 不能重复")
		}
		seen[item.ID] = true
		if item.CreditMicrocredits <= 0 || (item.CNYAmountMinor <= 0 && item.USDAmountMinor <= 0) {
			return PaymentSetting{}, BadAuthRequest("充值套餐积分和至少一种货币价格必须大于 0")
		}
	}
	req.Generic.Currency = strings.ToUpper(strings.TrimSpace(req.Generic.Currency))
	if req.Generic.Currency == "" {
		req.Generic.Currency = "USD"
	}
	if req.Generic.APIKeyHeader == "" {
		req.Generic.APIKeyHeader = "Authorization"
	}
	return req, nil
}

func validatePaymentSetting(value PaymentSetting) error {
	if !value.Enabled {
		return nil
	}
	if value.BaseURL == "" {
		return BadAuthRequest("启用支付前请配置外部可访问的回调基础地址")
	}
	baseURL, _ := url.Parse(value.BaseURL)
	if baseURL == nil || baseURL.Scheme != "https" {
		return BadAuthRequest("生产支付回调基础地址必须使用 HTTPS")
	}
	if len(value.Packages) == 0 {
		return BadAuthRequest("启用支付前请至少配置一个充值套餐")
	}
	if value.Alipay.Enabled && (value.Alipay.AppID == "" || value.Alipay.MerchantPrivateKey == "" || value.Alipay.AlipayPublicKey == "") {
		return BadAuthRequest("支付宝 AppID、应用私钥和支付宝公钥必须完整配置")
	}
	if value.WeChat.Enabled && (value.WeChat.AppID == "" || value.WeChat.MerchantID == "" || value.WeChat.MerchantSerialNo == "" || value.WeChat.MerchantPrivateKey == "" || len(value.WeChat.APIV3Key) != 32 || value.WeChat.PlatformPublicKeyID == "" || value.WeChat.PlatformPublicKey == "") {
		return BadAuthRequest("微信支付 AppID、商户号、证书序列号、商户私钥、32 位 APIv3 密钥和平台公钥必须完整配置")
	}
	if value.Stripe.Enabled && (value.Stripe.SecretKey == "" || value.Stripe.WebhookSecret == "") {
		return BadAuthRequest("Stripe Secret Key 和 Webhook Secret 必须完整配置")
	}
	if value.PayPal.Enabled && (value.PayPal.ClientID == "" || value.PayPal.ClientSecret == "" || value.PayPal.WebhookID == "") {
		return BadAuthRequest("PayPal Client ID、Client Secret 和 Webhook ID 必须完整配置")
	}
	if value.Generic.Enabled {
		parsed, err := url.Parse(strings.TrimSpace(value.Generic.CreateURL))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || value.Generic.APIKey == "" || value.Generic.WebhookSecret == "" {
			return BadAuthRequest("通用支付创建地址必须使用 HTTPS，并配置 API Key 和 Webhook Secret")
		}
		if !validPaymentAPIKeyHeader(value.Generic.APIKeyHeader) {
			return BadAuthRequest("通用支付 API Key Header 不合法或不安全")
		}
	}
	if !value.Alipay.Enabled && !value.WeChat.Enabled && !value.Stripe.Enabled && !value.PayPal.Enabled && !value.Generic.Enabled {
		return BadAuthRequest("启用支付前请至少启用一个支付渠道")
	}
	return nil
}

func validPaymentAPIKeyHeader(value string) bool {
	value = http.CanonicalHeaderKey(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	switch value {
	case "Host", "Content-Length", "Connection", "Transfer-Encoding", "Trailer", "Upgrade":
		return false
	}
	for _, char := range value {
		if !(char >= 'A' && char <= 'Z') && !(char >= 'a' && char <= 'z') && !(char >= '0' && char <= '9') && char != '-' {
			return false
		}
	}
	return true
}

func keepSecret(next *string, current string) {
	if strings.TrimSpace(*next) == "" {
		*next = current
	}
}

func publicPaymentSetting(record *model.SystemSetting, value PaymentSetting) *PaymentSetting {
	public := value
	public.Alipay.HasMerchantPrivateKey = value.Alipay.MerchantPrivateKey != ""
	public.WeChat.HasMerchantPrivateKey = value.WeChat.MerchantPrivateKey != ""
	public.WeChat.HasAPIV3Key = value.WeChat.APIV3Key != ""
	public.Stripe.HasSecretKey = value.Stripe.SecretKey != ""
	public.Stripe.HasWebhookSecret = value.Stripe.WebhookSecret != ""
	public.PayPal.HasClientSecret = value.PayPal.ClientSecret != ""
	public.Generic.HasAPIKey = value.Generic.APIKey != ""
	public.Generic.HasWebhookSecret = value.Generic.WebhookSecret != ""
	public.Alipay.MerchantPrivateKey = ""
	public.WeChat.MerchantPrivateKey = ""
	public.WeChat.APIV3Key = ""
	public.Stripe.SecretKey = ""
	public.Stripe.WebhookSecret = ""
	public.PayPal.ClientSecret = ""
	public.Generic.APIKey = ""
	public.Generic.WebhookSecret = ""
	if record != nil {
		public.UpdatedBy = record.UpdatedBy
		public.UpdatedAt = &record.UpdatedAt
	}
	return &public
}

func paymentPackagePrice(item PaymentPackage, provider string, genericCurrency string) (string, int64, error) {
	switch provider {
	case "alipay", "wechat":
		if item.CNYAmountMinor <= 0 {
			return "", 0, BadAuthRequest("该套餐未配置人民币价格")
		}
		return "CNY", item.CNYAmountMinor, nil
	case "stripe", "paypal":
		if item.USDAmountMinor <= 0 {
			return "", 0, BadAuthRequest("该套餐未配置美元价格")
		}
		return "USD", item.USDAmountMinor, nil
	case "generic":
		currency := strings.ToUpper(genericCurrency)
		if currency == "CNY" && item.CNYAmountMinor > 0 {
			return currency, item.CNYAmountMinor, nil
		}
		if currency == "USD" && item.USDAmountMinor > 0 {
			return currency, item.USDAmountMinor, nil
		}
		return "", 0, fmt.Errorf("通用支付当前货币 %s 未配置套餐价格", currency)
	default:
		return "", 0, BadAuthRequest("不支持的支付渠道")
	}
}
