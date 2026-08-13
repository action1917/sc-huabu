package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestVerifyStripeSignature(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"type":"checkout.session.completed"}`)
	now := time.Unix(1_800_000_000, 0)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "." + string(body)))
	header := "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	if !verifyStripeSignature(secret, header, body, now) {
		t.Fatal("expected valid Stripe signature")
	}
	if verifyStripeSignature(secret, header, []byte(`{"changed":true}`), now) {
		t.Fatal("changed body must not verify")
	}
	if verifyStripeSignature(secret, header, body, now.Add(6*time.Minute)) {
		t.Fatal("expired signature must not verify")
	}
}
func TestParseAmountMinorRejectsExtraPrecision(t *testing.T) {
	if _, err := parseAmountMinor("1.001"); err == nil {
		t.Fatal("expected extra precision to be rejected")
	}
	value, err := parseAmountMinor("12.30")
	if err != nil || value != 1230 {
		t.Fatalf("value = %d, err = %v", value, err)
	}
}

func TestValidatePaymentSettingRequiresHTTPS(t *testing.T) {
	setting := PaymentSetting{Enabled: true, BaseURL: "http://example.com", Packages: []PaymentPackage{{ID: "one", Name: "One", CreditMicrocredits: 1, USDAmountMinor: 1}}, Stripe: StripePaymentSetting{Enabled: true, SecretKey: "sk", WebhookSecret: "whsec"}}
	if err := validatePaymentSetting(setting); err == nil {
		t.Fatal("expected HTTP callback base URL to be rejected")
	}
	setting.BaseURL = "https://example.com"
	if err := validatePaymentSetting(setting); err != nil {
		t.Fatalf("HTTPS setting rejected: %v", err)
	}
}
