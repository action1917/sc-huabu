package repository

import (
	"testing"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPurchaseInspirationTransfersCreditsOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.Inspiration{}, &model.InspirationPurchase{}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	if err := db.Create(&model.CreditAccount{UserID: "buyer", AvailableMicrocredits: 50_000_000}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "seller"}).Error; err != nil {
		t.Fatal(err)
	}
	item := &model.Inspiration{ID: "inspiration-1", OwnerUserID: "seller", Title: "测试灵感", PriceMicrocredits: 10_000_000, Status: "published"}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	purchased, err := repo.PurchaseInspiration(item, "buyer")
	if err != nil || !purchased {
		t.Fatalf("first purchase = %v, %v", purchased, err)
	}
	purchased, err = repo.PurchaseInspiration(item, "buyer")
	if err != nil || purchased {
		t.Fatalf("second purchase = %v, %v", purchased, err)
	}
	var buyer, seller model.CreditAccount
	if err := db.First(&buyer, "user_id = ?", "buyer").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&seller, "user_id = ?", "seller").Error; err != nil {
		t.Fatal(err)
	}
	if buyer.AvailableMicrocredits != 40_000_000 || seller.AvailableMicrocredits != 10_000_000 {
		t.Fatalf("balances buyer=%d seller=%d", buyer.AvailableMicrocredits, seller.AvailableMicrocredits)
	}
	var entries int64
	if err := db.Model(&model.CreditLedgerEntry{}).Count(&entries).Error; err != nil {
		t.Fatal(err)
	}
	if entries != 2 {
		t.Fatalf("ledger entries = %d, want 2", entries)
	}
	var buyerEntry, sellerEntry model.CreditLedgerEntry
	if err := db.First(&buyerEntry, "user_id = ? AND type = ?", "buyer", model.CreditLedgerInspirationPurchase).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&sellerEntry, "user_id = ? AND type = ?", "seller", model.CreditLedgerInspirationSale).Error; err != nil {
		t.Fatal(err)
	}
	if buyerEntry.AmountMicrocredits != -10_000_000 || buyerEntry.AvailableAfterMicrocredits != 40_000_000 || buyerEntry.Scene != "inspiration" || buyerEntry.Note != "购买灵感：测试灵感" {
		t.Fatalf("unexpected buyer ledger: %+v", buyerEntry)
	}
	if sellerEntry.AmountMicrocredits != 10_000_000 || sellerEntry.AvailableAfterMicrocredits != 10_000_000 || sellerEntry.Scene != "inspiration" || sellerEntry.Note != "灵感售出：测试灵感" {
		t.Fatalf("unexpected seller ledger: %+v", sellerEntry)
	}
}

func TestPurchaseInspirationRejectsInsufficientCredits(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.Inspiration{}, &model.InspirationPurchase{}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	if err := db.Create(&model.CreditAccount{UserID: "buyer", AvailableMicrocredits: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "seller"}).Error; err != nil {
		t.Fatal(err)
	}
	item := &model.Inspiration{ID: "inspiration-2", OwnerUserID: "seller", Title: "收费灵感", PriceMicrocredits: 10, Status: "published"}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PurchaseInspiration(item, "buyer"); err != ErrInsufficientCredits {
		t.Fatalf("err = %v, want insufficient", err)
	}
	var purchases int64
	if err := db.Model(&model.InspirationPurchase{}).Count(&purchases).Error; err != nil {
		t.Fatal(err)
	}
	if purchases != 0 {
		t.Fatalf("purchases = %d, want 0", purchases)
	}
	var entries int64
	if err := db.Model(&model.CreditLedgerEntry{}).Count(&entries).Error; err != nil {
		t.Fatal(err)
	}
	if entries != 0 {
		t.Fatalf("ledger entries = %d, want 0", entries)
	}
	var seller model.CreditAccount
	if err := db.First(&seller, "user_id = ?", "seller").Error; err != nil {
		t.Fatal(err)
	}
	if seller.AvailableMicrocredits != 0 {
		t.Fatalf("seller balance = %d, want 0", seller.AvailableMicrocredits)
	}
}

func TestSetInspirationFavoriteIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.InspirationFavorite{}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	if err := repo.SetInspirationFavorite("user-1", "inspiration-1", true); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetInspirationFavorite("user-1", "inspiration-1", true); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.InspirationFavorite{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("favorite count = %d, want 1", count)
	}
	if err := repo.SetInspirationFavorite("user-1", "inspiration-1", false); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.InspirationFavorite{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("favorite count after delete = %d, want 0", count)
	}
}
