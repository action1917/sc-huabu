package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

func (s *Service) createStripeCheckout(ctx context.Context, setting PaymentSetting, order *model.PaymentOrder) (*paymentCheckout, error) {
	form := url.Values{
		"mode": {"payment"},
		"success_url": {setting.BaseURL + "/wallet?paymentOrder=" + order.ID},
		"cancel_url": {setting.BaseURL + "/wallet?paymentOrder=" + order.ID},
		"client_reference_id": {order.ID},
		"metadata[order_id]": {order.ID},
		"payment_intent_data[metadata][order_id]": {order.ID},
		"line_items[0][quantity]": {"1"},
		"line_items[0][price_data][currency]": {strings.ToLower(order.Currency)},
		"line_items[0][price_data][unit_amount]": {strconv.FormatInt(order.AmountMinor, 10)},
		"line_items[0][price_data][product_data][name]": {order.Subject},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil { return nil, err }
	request.Header.Set("Authorization", "Bearer "+setting.Stripe.SecretKey)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := OutboundHTTPClient(20 * time.Second).Do(request)
	if err != nil { return nil, err }
	body, err := limitedResponseBody(response)
	if err != nil { return nil, err }
	var result struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.ID == "" || result.URL == "" { return nil, errors.New("Stripe 下单响应无效") }
	var expires *time.Time
	if result.ExpiresAt > 0 {
		value := time.Unix(result.ExpiresAt, 0)
		expires = &value
	}
	return &paymentCheckout{ProviderOrderID: result.ID, Mode: "redirect", URL: result.URL, ExpiresAt: expires}, nil
}

func (s *Service) HandleStripeWebhook(signatureHeader string, body []byte) error {
	_, setting, err := s.readPaymentSetting()
	if err != nil { return err }
	if !setting.Enabled || !setting.Stripe.Enabled { return errors.New("Stripe 支付未启用") }
	if !verifyStripeSignature(setting.Stripe.WebhookSecret, signatureHeader, body, time.Now()) { return errors.New("Stripe Webhook 验签失败") }
	var event struct {
		ID, Type string
		Data struct { Object struct {
			ID string; ClientReferenceID string `json:"client_reference_id"`; PaymentStatus string `json:"payment_status"`
			PaymentIntent string `json:"payment_intent"`; Currency string; AmountTotal int64 `json:"amount_total"`
			Metadata map[string]string `json:"metadata"`
		} `json:"object"` } `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil { return err }
	if event.Type != "checkout.session.completed" && event.Type != "checkout.session.async_payment_succeeded" { return nil }
	object := event.Data.Object
	if object.PaymentStatus != "paid" { return nil }
	orderID := firstNonEmpty(object.Metadata["order_id"], object.ClientReferenceID)
	order, err := s.repo.PaymentOrder(orderID)
	if err != nil { return err }
	_, err = s.completePayment(order, object.ID, firstNonEmpty(object.PaymentIntent, event.ID), strings.ToUpper(object.Currency), object.AmountTotal)
	return err
}

func verifyStripeSignature(secret string, header string, body []byte, now time.Time) bool {
	var timestamp int64
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok { continue }
		if key == "t" { timestamp, _ = strconv.ParseInt(value, 10, 64) }
		if key == "v1" { signatures = append(signatures, value) }
	}
	if timestamp <= 0 || len(signatures) == 0 || now.Sub(time.Unix(timestamp, 0)) > 5*time.Minute || time.Unix(timestamp, 0).Sub(now) > 5*time.Minute { return false }
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "." + string(body)))
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		candidate, err := hex.DecodeString(signature)
		if err == nil && hmac.Equal(candidate, expected) { return true }
	}
	return false
}

func (s *Service) createPayPalCheckout(ctx context.Context, setting PaymentSetting, order *model.PaymentOrder) (*paymentCheckout, error) {
	config := setting.PayPal
	baseURL := paypalBaseURL(config.Sandbox)
	token, err := paypalAccessToken(ctx, config)
	if err != nil { return nil, err }
	payload, _ := json.Marshal(map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{{"reference_id": order.ID, "custom_id": order.ID, "description": order.Subject, "amount": map[string]string{"currency_code": order.Currency, "value": minorAmount(order.AmountMinor)}}},
		"payment_source": map[string]any{"paypal": map[string]any{"experience_context": map[string]string{"user_action": "PAY_NOW", "return_url": setting.BaseURL + "/api/payments/paypal/return?order=" + order.ID, "cancel_url": setting.BaseURL + "/wallet?paymentOrder=" + order.ID}}},
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v2/checkout/orders", bytes.NewReader(payload))
	if err != nil { return nil, err }
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("PayPal-Request-Id", order.ID)
	response, err := OutboundHTTPClient(20 * time.Second).Do(request)
	if err != nil { return nil, err }
	body, err := limitedResponseBody(response)
	if err != nil { return nil, err }
	var result struct { ID string; Links []struct { Href, Rel string } }
	if err := json.Unmarshal(body, &result); err != nil || result.ID == "" { return nil, errors.New("PayPal 下单响应无效") }
	approval := ""
	for _, link := range result.Links { if link.Rel == "payer-action" || link.Rel == "approve" { approval = link.Href; break } }
	if approval == "" { return nil, errors.New("PayPal 下单响应缺少付款链接") }
	expires := time.Now().Add(3 * time.Hour)
	return &paymentCheckout{ProviderOrderID: result.ID, Mode: "redirect", URL: approval, ExpiresAt: &expires}, nil
}

func (s *Service) CapturePayPalOrder(ctx context.Context, localOrderID string, providerOrderID string) (*model.PaymentOrder, error) {
	_, setting, err := s.readPaymentSetting()
	if err != nil { return nil, err }
	order, err := s.repo.PaymentOrder(localOrderID)
	if err != nil { return nil, err }
	if order.Provider != "paypal" || order.ProviderOrderID != providerOrderID { return nil, errors.New("PayPal 订单号不匹配") }
	if order.Status == model.PaymentStatusPaid { return order, nil }
	token, err := paypalAccessToken(ctx, setting.PayPal)
	if err != nil { return nil, err }
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, paypalBaseURL(setting.PayPal.Sandbox)+"/v2/checkout/orders/"+url.PathEscape(providerOrderID)+"/capture", bytes.NewReader([]byte(`{}`)))
	if err != nil { return nil, err }
	request.Header.Set("Authorization", "Bearer "+token); request.Header.Set("Content-Type", "application/json")
	response, err := OutboundHTTPClient(20 * time.Second).Do(request)
	if err != nil { return nil, err }
	body, err := limitedResponseBody(response)
	if err != nil { return nil, err }
	return s.completePayPalPayload(order, body)
}

func (s *Service) HandlePayPalWebhook(ctx context.Context, headers http.Header, body []byte) error {
	_, setting, err := s.readPaymentSetting()
	if err != nil { return err }
	if !setting.Enabled || !setting.PayPal.Enabled { return errors.New("PayPal 支付未启用") }
	if err := verifyPayPalWebhook(ctx, setting.PayPal, headers, body); err != nil { return err }
	var event struct { Type string `json:"event_type"`; Resource json.RawMessage `json:"resource"` }
	if err := json.Unmarshal(body, &event); err != nil { return err }
	if event.Type != "PAYMENT.CAPTURE.COMPLETED" { return nil }
	var capture struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		CustomID  string `json:"custom_id"`
		InvoiceID string `json:"invoice_id"`
		Amount    struct {
			CurrencyCode string `json:"currency_code"`
			Value        string `json:"value"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(event.Resource, &capture); err != nil { return err }
	orderID := firstNonEmpty(capture.CustomID, capture.InvoiceID)
	order, err := s.repo.PaymentOrder(orderID)
	if err != nil { return err }
	amount, err := parseAmountMinor(capture.Amount.Value); if err != nil { return err }
	_, err = s.completePayment(order, order.ProviderOrderID, capture.ID, capture.Amount.CurrencyCode, amount)
	return err
}

func (s *Service) completePayPalPayload(order *model.PaymentOrder, body []byte) (*model.PaymentOrder, error) {
	var payload struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PurchaseUnits []struct {
			ReferenceID string `json:"reference_id"`
			CustomID    string `json:"custom_id"`
			Payments    struct {
				Captures []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Amount struct {
						CurrencyCode string `json:"currency_code"`
						Value        string `json:"value"`
					} `json:"amount"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}
	if err := json.Unmarshal(body, &payload); err != nil { return nil, err }
	for _, unit := range payload.PurchaseUnits { for _, capture := range unit.Payments.Captures {
		if capture.Status != "COMPLETED" { continue }
		amount, err := parseAmountMinor(capture.Amount.Value); if err != nil { return nil, err }
		return s.completePayment(order, payload.ID, capture.ID, capture.Amount.CurrencyCode, amount)
	} }
	return nil, errors.New("PayPal 订单尚未完成扣款")
}

func paypalAccessToken(ctx context.Context, config PayPalPaymentSetting) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, paypalBaseURL(config.Sandbox)+"/v1/oauth2/token", strings.NewReader("grant_type=client_credentials"))
	if err != nil { return "", err }
	request.SetBasicAuth(config.ClientID, config.ClientSecret); request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := OutboundHTTPClient(20 * time.Second).Do(request); if err != nil { return "", err }
	body, err := limitedResponseBody(response); if err != nil { return "", err }
	var result struct { AccessToken string `json:"access_token"` }
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" { return "", errors.New("PayPal 无法获取访问令牌") }
	return result.AccessToken, nil
}

func verifyPayPalWebhook(ctx context.Context, config PayPalPaymentSetting, headers http.Header, body []byte) error {
	for _, key := range []string{"PayPal-Transmission-Id", "PayPal-Transmission-Time", "PayPal-Cert-Url", "PayPal-Auth-Algo", "PayPal-Transmission-Sig"} { if headers.Get(key) == "" { return errors.New("PayPal Webhook 缺少验签头") } }
	if !json.Valid(body) { return errors.New("PayPal Webhook JSON 无效") }
	token, err := paypalAccessToken(ctx, config); if err != nil { return err }
	payload, _ := json.Marshal(map[string]any{
		"transmission_id": headers.Get("PayPal-Transmission-Id"), "transmission_time": headers.Get("PayPal-Transmission-Time"),
		"cert_url": headers.Get("PayPal-Cert-Url"), "auth_algo": headers.Get("PayPal-Auth-Algo"), "transmission_sig": headers.Get("PayPal-Transmission-Sig"),
		"webhook_id": config.WebhookID, "webhook_event": json.RawMessage(body),
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, paypalBaseURL(config.Sandbox)+"/v1/notifications/verify-webhook-signature", bytes.NewReader(payload)); if err != nil { return err }
	request.Header.Set("Authorization", "Bearer "+token); request.Header.Set("Content-Type", "application/json")
	response, err := OutboundHTTPClient(20 * time.Second).Do(request); if err != nil { return err }
	data, err := limitedResponseBody(response); if err != nil { return err }
	var result struct { VerificationStatus string `json:"verification_status"` }; if json.Unmarshal(data, &result) != nil || result.VerificationStatus != "SUCCESS" { return errors.New("PayPal Webhook 验签失败") }
	return nil
}

func paypalBaseURL(sandbox bool) string { if sandbox { return "https://api-m.sandbox.paypal.com" }; return "https://api-m.paypal.com" }

func (s *Service) createGenericCheckout(ctx context.Context, setting PaymentSetting, order *model.PaymentOrder) (*paymentCheckout, error) {
	config := setting.Generic
	if _, err := ValidateCustomRelayURL(config.CreateURL); err != nil { return nil, err }
	payload, _ := json.Marshal(map[string]any{"orderId": order.ID, "amountMinor": order.AmountMinor, "currency": order.Currency, "description": order.Subject, "returnUrl": setting.BaseURL + "/wallet?paymentOrder=" + order.ID, "webhookUrl": setting.BaseURL + "/api/payments/webhooks/generic"})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.CreateURL, bytes.NewReader(payload)); if err != nil { return nil, err }
	request.Header.Set("Content-Type", "application/json"); request.Header.Set(config.APIKeyHeader, config.APIKey)
	response, err := CustomRelayHTTPClient(20 * time.Second).Do(request); if err != nil { return nil, err }
	body, err := limitedResponseBody(response); if err != nil { return nil, err }
	var result struct {
		ProviderOrderID string     `json:"providerOrderId"`
		CheckoutURL     string     `json:"checkoutUrl"`
		ExpiresAt       *time.Time `json:"expiresAt"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.ProviderOrderID == "" || result.CheckoutURL == "" { return nil, errors.New("通用支付下单响应必须包含 providerOrderId 和 checkoutUrl") }
	checkoutURL, err := url.Parse(result.CheckoutURL)
	if err != nil || checkoutURL.Scheme != "https" || checkoutURL.Host == "" { return nil, errors.New("通用支付收银台地址必须使用 HTTPS") }
	return &paymentCheckout{ProviderOrderID: result.ProviderOrderID, Mode: "redirect", URL: result.CheckoutURL, ExpiresAt: result.ExpiresAt}, nil
}

func (s *Service) HandleGenericWebhook(signature string, timestamp string, body []byte) error {
	_, setting, err := s.readPaymentSetting(); if err != nil { return err }
	if !setting.Enabled || !setting.Generic.Enabled { return errors.New("通用支付未启用") }
	unixTime, err := strconv.ParseInt(timestamp, 10, 64); if err != nil || time.Since(time.Unix(unixTime, 0)) > 5*time.Minute || time.Unix(unixTime, 0).Sub(time.Now()) > 5*time.Minute { return errors.New("通用支付 Webhook 时间戳无效") }
	mac := hmac.New(sha256.New, []byte(setting.Generic.WebhookSecret)); _, _ = mac.Write([]byte(timestamp+"."+string(body)))
	candidate, err := hex.DecodeString(signature); if err != nil || !hmac.Equal(candidate, mac.Sum(nil)) { return errors.New("通用支付 Webhook 验签失败") }
	var event struct {
		Event           string `json:"event"`
		OrderID         string `json:"orderId"`
		ProviderOrderID string `json:"providerOrderId"`
		TransactionID   string `json:"transactionId"`
		Currency        string `json:"currency"`
		AmountMinor     int64  `json:"amountMinor"`
	}
	if err := json.Unmarshal(body, &event); err != nil { return err }
	if event.Event != "payment.succeeded" { return nil }
	order, err := s.repo.PaymentOrder(event.OrderID); if err != nil { return err }
	_, err = s.completePayment(order, event.ProviderOrderID, event.TransactionID, event.Currency, event.AmountMinor)
	return err
}
