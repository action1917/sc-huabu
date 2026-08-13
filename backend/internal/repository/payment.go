package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPaymentOrderConflict = errors.New("payment order state conflict")

func (r *Repository) CreatePaymentOrder(order *model.PaymentOrder) error {
	return r.db.Create(order).Error
}

func (r *Repository) UpdatePaymentCheckout(id string, providerOrderID string, mode string, checkoutURL string, expiresAt *time.Time) error {
	return r.db.Model(&model.PaymentOrder{}).Where("id = ? AND status = ?", id, model.PaymentStatusPending).Updates(map[string]any{
		"provider_order_id": providerOrderID,
		"checkout_mode":     mode,
		"checkout_url":      checkoutURL,
		"expires_at":        expiresAt,
		"error":             "",
	}).Error
}

func (r *Repository) FailPaymentOrder(id string, message string) error {
	return r.db.Model(&model.PaymentOrder{}).Where("id = ? AND status = ?", id, model.PaymentStatusPending).Updates(map[string]any{
		"status": model.PaymentStatusFailed,
		"error":  message,
	}).Error
}

func (r *Repository) PaymentOrderForUser(userID string, id string) (*model.PaymentOrder, error) {
	var order model.PaymentOrder
	if err := r.db.First(&order, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Repository) PaymentOrder(id string) (*model.PaymentOrder, error) {
	var order model.PaymentOrder
	if err := r.db.First(&order, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Repository) PaymentOrderByProviderID(provider string, providerOrderID string) (*model.PaymentOrder, error) {
	var order model.PaymentOrder
	if err := r.db.First(&order, "provider = ? AND provider_order_id = ?", provider, providerOrderID).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Repository) PaymentOrdersForUser(userID string, limit int) ([]model.PaymentOrder, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var orders []model.PaymentOrder
	err := r.db.Where("user_id = ?", userID).Order("created_at desc").Limit(limit).Find(&orders).Error
	return orders, err
}

// CompletePaymentOrder atomically marks the provider order paid and grants credits once.
func (r *Repository) CompletePaymentOrder(id string, providerTransactionID string, ledgerID string) (*model.PaymentOrder, bool, error) {
	var completed model.PaymentOrder
	credited := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var order model.PaymentOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ?", id).Error; err != nil {
			return err
		}
		if order.Status == model.PaymentStatusPaid {
			completed = order
			return nil
		}
		if order.Status != model.PaymentStatusPending {
			return ErrPaymentOrderConflict
		}
		now := time.Now()
		result := tx.Model(&model.PaymentOrder{}).Where("id = ? AND status = ?", order.ID, model.PaymentStatusPending).Updates(map[string]any{
			"status":                  model.PaymentStatusPaid,
			"provider_transaction_id": providerTransactionID,
			"paid_at":                 now,
			"error":                   "",
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPaymentOrderConflict
		}

		account := model.CreditAccount{UserID: order.UserID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CreditAccount{}).Where("user_id = ?", order.UserID).Updates(map[string]any{
			"available_microcredits": gorm.Expr("available_microcredits + ?", order.CreditMicrocredits),
			"version":                gorm.Expr("version + 1"),
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&account, "user_id = ?", order.UserID).Error; err != nil {
			return err
		}
		referenceKey := "payment:" + order.ID
		entry := model.CreditLedgerEntry{
			ID:                         ledgerID,
			UserID:                     order.UserID,
			Type:                       model.CreditLedgerPayment,
			AmountMicrocredits:         order.CreditMicrocredits,
			AvailableDeltaMicrocredits: order.CreditMicrocredits,
			AvailableAfterMicrocredits: account.AvailableMicrocredits,
			ReservedAfterMicrocredits:  account.ReservedMicrocredits,
			Note:                       order.Subject,
			ReferenceKey:               &referenceKey,
			CreatedAt:                  now,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		order.Status = model.PaymentStatusPaid
		order.ProviderTransactionID = providerTransactionID
		order.PaidAt = &now
		completed = order
		credited = true
		return nil
	})
	return &completed, credited, err
}
