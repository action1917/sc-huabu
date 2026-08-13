package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

type PublicPaymentProvider struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Mode     string `json:"mode"`
}

type PublicPaymentCatalog struct {
	Enabled   bool                    `json:"enabled"`
	Packages  []PaymentPackage        `json:"packages"`
	Providers []PublicPaymentProvider `json:"providers"`
}

type CreatePaymentRequest struct {
	Provider  string `json:"provider"`
	PackageID string `json:"packageId"`
}

type paymentCheckout struct {
	ProviderOrderID string
	Mode            string
	URL             string
	ExpiresAt       *time.Time
}

func (s *Service) PaymentCatalog(user *model.User) (*PublicPaymentCatalog, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureCredits); err != nil {
		return nil, err
	}
	_, setting, err := s.readPaymentSetting()
	if err != nil {
		return nil, err
	}
	catalog := &PublicPaymentCatalog{Enabled: setting.Enabled, Packages: setting.Packages, Providers: []PublicPaymentProvider{}}
	if !setting.Enabled {
		return catalog, nil
	}
	if setting.Alipay.Enabled { catalog.Providers = append(catalog.Providers, PublicPaymentProvider{Key: "alipay", Name: "支付宝", Currency: "CNY", Mode: "redirect"}) }
	if setting.WeChat.Enabled { catalog.Providers = append(catalog.Providers, PublicPaymentProvider{Key: "wechat", Name: "微信支付", Currency: "CNY", Mode: "qrcode"}) }
	if setting.Stripe.Enabled { catalog.Providers = append(catalog.Providers, PublicPaymentProvider{Key: "stripe", Name: "Stripe", Currency: "USD", Mode: "redirect"}) }
	if setting.PayPal.Enabled { catalog.Providers = append(catalog.Providers, PublicPaymentProvider{Key: "paypal", Name: "PayPal", Currency: "USD", Mode: "redirect"}) }
	if setting.Generic.Enabled { catalog.Providers = append(catalog.Providers, PublicPaymentProvider{Key: "generic", Name: firstNonEmpty(setting.Generic.Name, "其他国际支付"), Currency: setting.Generic.Currency, Mode: "redirect"}) }
	return catalog, nil
}

func (s *Service) CreatePayment(ctx context.Context, user *model.User, req CreatePaymentRequest) (*model.PaymentOrder, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureCredits); err != nil {
		return nil, err
	}
	_, setting, err := s.readPaymentSetting()
	if err != nil {
		return nil, err
	}
	if !setting.Enabled {
		return nil, BadAuthRequest("在线支付暂未开放")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if !paymentProviderEnabled(setting, provider) {
		return nil, BadAuthRequest("支付渠道未启用")
	}
	var pack *PaymentPackage
	for index := range setting.Packages {
		if setting.Packages[index].ID == strings.TrimSpace(req.PackageID) {
			pack = &setting.Packages[index]
			break
		}
	}
	if pack == nil {
		return nil, BadAuthRequest("充值套餐不存在")
	}
	currency, amount, err := paymentPackagePrice(*pack, provider, setting.Generic.Currency)
	if err != nil {
		return nil, err
	}
	order := &model.PaymentOrder{
		ID: newID(), UserID: user.ID, Provider: provider, PackageID: pack.ID,
		Subject: truncateRunes("速创积分充值 - "+pack.Name, 160), Currency: currency,
		AmountMinor: amount, CreditMicrocredits: pack.CreditMicrocredits, Status: model.PaymentStatusPending,
	}
	if err := s.repo.CreatePaymentOrder(order); err != nil {
		return nil, err
	}
	checkout, err := s.createPaymentCheckout(ctx, setting, order)
	if err != nil {
		_ = s.repo.FailPaymentOrder(order.ID, truncateRunes(err.Error(), 1000))
		return nil, err
	}
	if err := s.repo.UpdatePaymentCheckout(order.ID, checkout.ProviderOrderID, checkout.Mode, checkout.URL, checkout.ExpiresAt); err != nil {
		return nil, err
	}
	order.ProviderOrderID = checkout.ProviderOrderID
	order.CheckoutMode = checkout.Mode
	order.CheckoutURL = checkout.URL
	order.ExpiresAt = checkout.ExpiresAt
	return order, nil
}

func (s *Service) PaymentOrder(user *model.User, id string) (*model.PaymentOrder, error) {
	if user == nil { return nil, Unauthorized("请先登录") }
	return s.repo.PaymentOrderForUser(user.ID, strings.TrimSpace(id))
}

func (s *Service) PaymentOrders(user *model.User, limit int) ([]model.PaymentOrder, error) {
	if user == nil { return nil, Unauthorized("请先登录") }
	return s.repo.PaymentOrdersForUser(user.ID, limit)
}

func (s *Service) completePayment(order *model.PaymentOrder, providerOrderID string, providerTransactionID string, currency string, amountMinor int64) (*model.PaymentOrder, error) {
	if order == nil || order.ProviderOrderID == "" || order.ProviderOrderID != providerOrderID {
		return nil, errors.New("支付平台订单号不匹配")
	}
	if !strings.EqualFold(order.Currency, currency) || order.AmountMinor != amountMinor {
		return nil, errors.New("支付金额或币种与本地订单不匹配")
	}
	completed, credited, err := s.repo.CompletePaymentOrder(order.ID, providerTransactionID, newID())
	if errors.Is(err, repository.ErrPaymentOrderConflict) {
		return nil, BadAuthRequest("支付订单当前状态不能入账")
	}
	if err != nil {
		return nil, err
	}
	_ = credited
	return completed, nil
}

func paymentProviderEnabled(setting PaymentSetting, provider string) bool {
	switch provider {
	case "alipay": return setting.Alipay.Enabled
	case "wechat": return setting.WeChat.Enabled
	case "stripe": return setting.Stripe.Enabled
	case "paypal": return setting.PayPal.Enabled
	case "generic": return setting.Generic.Enabled
	default: return false
	}
}

func (s *Service) createPaymentCheckout(ctx context.Context, setting PaymentSetting, order *model.PaymentOrder) (*paymentCheckout, error) {
	switch order.Provider {
	case "alipay": return s.createAlipayCheckout(setting, order)
	case "wechat": return s.createWeChatCheckout(ctx, setting, order)
	case "stripe": return s.createStripeCheckout(ctx, setting, order)
	case "paypal": return s.createPayPalCheckout(ctx, setting, order)
	case "generic": return s.createGenericCheckout(ctx, setting, order)
	default: return nil, BadAuthRequest("不支持的支付渠道")
	}
}

func paymentHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func limitedResponseBody(response *http.Response) ([]byte, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil { return nil, err }
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("支付平台返回 HTTP %d：%s", response.StatusCode, truncateRunes(string(body), 500))
	}
	return body, nil
}
