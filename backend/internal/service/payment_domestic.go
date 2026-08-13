package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

func (s *Service) createAlipayCheckout(setting PaymentSetting, order *model.PaymentOrder) (*paymentCheckout, error) {
	config := setting.Alipay
	gateway := "https://openapi.alipay.com/gateway.do"
	if config.Sandbox { gateway = "https://openapi-sandbox.dl.alipaydev.com/gateway.do" }
	biz, _ := json.Marshal(map[string]any{
		"out_trade_no": order.ID,
		"total_amount": minorAmount(order.AmountMinor),
		"subject": order.Subject,
		"product_code": "FAST_INSTANT_TRADE_PAY",
		"timeout_express": "30m",
	})
	params := url.Values{
		"app_id": {config.AppID}, "method": {"alipay.trade.page.pay"}, "format": {"JSON"},
		"charset": {"utf-8"}, "sign_type": {"RSA2"}, "timestamp": {time.Now().Format("2006-01-02 15:04:05")},
		"version": {"1.0"}, "notify_url": {setting.BaseURL + "/api/payments/webhooks/alipay"},
		"return_url": {setting.BaseURL + "/wallet?paymentOrder=" + order.ID}, "biz_content": {string(biz)},
	}
	signature, err := rsaSignSHA256(config.MerchantPrivateKey, alipayCanonical(params))
	if err != nil { return nil, fmt.Errorf("支付宝应用私钥无效：%w", err) }
	params.Set("sign", signature)
	expires := time.Now().Add(30 * time.Minute)
	return &paymentCheckout{ProviderOrderID: order.ID, Mode: "redirect", URL: gateway + "?" + params.Encode(), ExpiresAt: &expires}, nil
}

func (s *Service) HandleAlipayWebhook(values url.Values) error {
	_, setting, err := s.readPaymentSetting()
	if err != nil { return err }
	if !setting.Enabled || !setting.Alipay.Enabled { return errors.New("支付宝支付未启用") }
	signature := values.Get("sign")
	if signature == "" || !rsaVerifySHA256(setting.Alipay.AlipayPublicKey, alipayCanonical(values), signature) {
		return errors.New("支付宝通知验签失败")
	}
	if values.Get("app_id") != setting.Alipay.AppID { return errors.New("支付宝通知 AppID 不匹配") }
	if setting.Alipay.SellerID != "" && values.Get("seller_id") != setting.Alipay.SellerID { return errors.New("支付宝通知收款账号不匹配") }
	status := values.Get("trade_status")
	if status != "TRADE_SUCCESS" && status != "TRADE_FINISHED" { return nil }
	order, err := s.repo.PaymentOrder(values.Get("out_trade_no"))
	if err != nil { return err }
	amount, err := parseAmountMinor(values.Get("total_amount"))
	if err != nil { return err }
	_, err = s.completePayment(order, values.Get("out_trade_no"), values.Get("trade_no"), "CNY", amount)
	return err
}

func alipayCanonical(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key, items := range values {
		if key != "sign" && key != "sign_type" && len(items) > 0 && items[0] != "" { keys = append(keys, key) }
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys { parts = append(parts, key+"="+values.Get(key)) }
	return strings.Join(parts, "&")
}

func (s *Service) createWeChatCheckout(ctx context.Context, setting PaymentSetting, order *model.PaymentOrder) (*paymentCheckout, error) {
	config := setting.WeChat
	payload, _ := json.Marshal(map[string]any{
		"appid": config.AppID, "mchid": config.MerchantID, "description": order.Subject,
		"out_trade_no": order.ID, "notify_url": setting.BaseURL + "/api/payments/webhooks/wechat",
		"amount": map[string]any{"total": order.AmountMinor, "currency": "CNY"},
	})
	endpoint := "https://api.mch.weixin.qq.com/v3/pay/transactions/native"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil { return nil, err }
	nonce := randomNonce()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := http.MethodPost + "\n/v3/pay/transactions/native\n" + timestamp + "\n" + nonce + "\n" + string(payload) + "\n"
	signature, err := rsaSignSHA256(config.MerchantPrivateKey, message)
	if err != nil { return nil, fmt.Errorf("微信商户私钥无效：%w", err) }
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`, config.MerchantID, nonce, timestamp, config.MerchantSerialNo, signature))
	response, err := paymentHTTPClient().Do(request)
	if err != nil { return nil, err }
	body, err := limitedResponseBody(response)
	if err != nil { return nil, err }
	if err := verifyWeChatResponse(config.PlatformPublicKey, response.Header, body); err != nil { return nil, err }
	var result struct { CodeURL string `json:"code_url"` }
	if err := json.Unmarshal(body, &result); err != nil || result.CodeURL == "" { return nil, errors.New("微信支付下单响应缺少 code_url") }
	expires := time.Now().Add(30 * time.Minute)
	return &paymentCheckout{ProviderOrderID: order.ID, Mode: "qrcode", URL: result.CodeURL, ExpiresAt: &expires}, nil
}

func (s *Service) HandleWeChatWebhook(headers http.Header, body []byte) error {
	_, setting, err := s.readPaymentSetting()
	if err != nil { return err }
	config := setting.WeChat
	if !setting.Enabled || !config.Enabled { return errors.New("微信支付未启用") }
	if headers.Get("Wechatpay-Serial") != config.PlatformPublicKeyID { return errors.New("微信支付平台公钥 ID 不匹配") }
	if err := verifyWeChatResponse(config.PlatformPublicKey, headers, body); err != nil { return err }
	var envelope struct {
		EventType string `json:"event_type"`
		Resource struct { Ciphertext, Nonce, AssociatedData string } `json:"resource"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil { return err }
	if envelope.EventType != "TRANSACTION.SUCCESS" { return nil }
	plain, err := decryptWeChatResource(config.APIV3Key, envelope.Resource.Nonce, envelope.Resource.AssociatedData, envelope.Resource.Ciphertext)
	if err != nil { return err }
	var transaction struct {
		AppID string `json:"appid"`; MerchantID string `json:"mchid"`; OutTradeNo string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`; TradeState string `json:"trade_state"`
		Amount struct { Total int64 `json:"total"`; Currency string `json:"currency"` } `json:"amount"`
	}
	if err := json.Unmarshal(plain, &transaction); err != nil { return err }
	if transaction.TradeState != "SUCCESS" { return nil }
	if transaction.AppID != config.AppID || transaction.MerchantID != config.MerchantID { return errors.New("微信支付通知商户信息不匹配") }
	order, err := s.repo.PaymentOrder(transaction.OutTradeNo)
	if err != nil { return err }
	_, err = s.completePayment(order, transaction.OutTradeNo, transaction.TransactionID, transaction.Amount.Currency, transaction.Amount.Total)
	return err
}

func verifyWeChatResponse(publicKeyPEM string, headers http.Header, body []byte) error {
	timestamp, nonce, signature := headers.Get("Wechatpay-Timestamp"), headers.Get("Wechatpay-Nonce"), headers.Get("Wechatpay-Signature")
	if timestamp == "" || nonce == "" || signature == "" { return errors.New("微信支付响应缺少验签头") }
	message := timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	if !rsaVerifySHA256(publicKeyPEM, message, signature) { return errors.New("微信支付响应验签失败") }
	return nil
}

func decryptWeChatResource(apiV3Key string, nonce string, associatedData string, ciphertext string) ([]byte, error) {
	if len(apiV3Key) != 32 { return nil, errors.New("微信支付 APIv3 密钥长度必须为 32") }
	block, err := aes.NewCipher([]byte(apiV3Key)); if err != nil { return nil, err }
	gcm, err := cipher.NewGCM(block); if err != nil { return nil, err }
	data, err := base64.StdEncoding.DecodeString(ciphertext); if err != nil { return nil, err }
	return gcm.Open(nil, []byte(nonce), data, []byte(associatedData))
}

func rsaSignSHA256(privateKeyPEM string, message string) (string, error) {
	block, _ := pem.Decode([]byte(normalizePEM(privateKeyPEM, "PRIVATE KEY")))
	if block == nil { return "", errors.New("PEM 私钥格式无效") }
	var key *rsa.PrivateKey
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil { key, _ = parsed.(*rsa.PrivateKey) }
	if key == nil { key, err = x509.ParsePKCS1PrivateKey(block.Bytes) }
	if err != nil || key == nil { return "", errors.New("RSA 私钥无法解析") }
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil { return "", err }
	return base64.StdEncoding.EncodeToString(signature), nil
}

func rsaVerifySHA256(publicKeyPEM string, message string, signature string) bool {
	block, _ := pem.Decode([]byte(normalizePEM(publicKeyPEM, "PUBLIC KEY")))
	if block == nil { return false }
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	var key *rsa.PublicKey
	if err == nil { key, _ = parsed.(*rsa.PublicKey) }
	if key == nil {
		if certificate, certErr := x509.ParseCertificate(block.Bytes); certErr == nil { key, _ = certificate.PublicKey.(*rsa.PublicKey) }
	}
	if key == nil { return false }
	sig, err := base64.StdEncoding.DecodeString(signature); if err != nil { return false }
	digest := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig) == nil
}

func normalizePEM(value string, kind string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\n", "\n"))
	if strings.Contains(value, "-----BEGIN") { return value }
	return "-----BEGIN " + kind + "-----\n" + value + "\n-----END " + kind + "-----"
}

func randomNonce() string {
	data := make([]byte, 16); _, _ = io.ReadFull(rand.Reader, data)
	return base64.RawURLEncoding.EncodeToString(data)
}

func minorAmount(value int64) string { return fmt.Sprintf("%d.%02d", value/100, value%100) }

func parseAmountMinor(value string) (int64, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) > 2 || len(parts) == 0 { return 0, errors.New("支付金额格式无效") }
	if len(parts) == 2 && len(parts[1]) > 2 { return 0, errors.New("支付金额最多两位小数") }
	whole, err := strconv.ParseInt(parts[0], 10, 64); if err != nil || whole < 0 { return 0, errors.New("支付金额格式无效") }
	fraction := "00"; if len(parts) == 2 { fraction = parts[1] + "00"; fraction = fraction[:2] }
	cents, err := strconv.ParseInt(fraction, 10, 64); if err != nil { return 0, errors.New("支付金额格式无效") }
	return whole*100 + cents, nil
}
