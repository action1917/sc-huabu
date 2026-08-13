package repository

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCompletePaymentOrderIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:payment-idempotent?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PaymentOrder{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	order := &model.PaymentOrder{ID: "order-1", UserID: "user-1", Provider: "stripe", ProviderOrderID: "cs_1", Subject: "充值", Currency: "USD", AmountMinor: 200, CreditMicrocredits: 100_000_000, Status: model.PaymentStatusPending, CreatedAt: time.Now()}
	if err := repo.CreatePaymentOrder(order); err != nil {
		t.Fatal(err)
	}

	completed, credited, err := repo.CompletePaymentOrder(order.ID, "pi_1", "ledger-1")
	if err != nil || !credited || completed.Status != model.PaymentStatusPaid {
		t.Fatalf("first completion = (%+v, %v, %v)", completed, credited, err)
	}
	completed, credited, err = repo.CompletePaymentOrder(order.ID, "pi_1", "ledger-2")
	if err != nil || credited || completed.Status != model.PaymentStatusPaid {
		t.Fatalf("duplicate completion = (%+v, %v, %v)", completed, credited, err)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != order.CreditMicrocredits {
		t.Fatalf("balance = %d, want %d", account.AvailableMicrocredits, order.CreditMicrocredits)
	}
	var count int64
	if err := db.Model(&model.CreditLedgerEntry{}).Where("user_id = ?", order.UserID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("ledger count = %d, want 1", count)
	}
}
