package repository

import (
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InspirationRow struct {
	model.Inspiration
	OwnerUsername    string `json:"ownerUsername" gorm:"column:owner_username"`
	OwnerDisplayName string `json:"ownerDisplayName" gorm:"column:owner_display_name"`
	Purchased        bool   `json:"purchased" gorm:"column:purchased"`
	Favorited        bool   `json:"favorited" gorm:"column:favorited"`
	FavoriteCount    int64  `json:"favoriteCount" gorm:"column:favorite_count"`
}

func (r *Repository) Inspirations(viewerUserID string, scope string, kind string, keyword string, limit int) ([]InspirationRow, error) {
	var rows []InspirationRow
	query := r.db.Table("inspirations i").
		Select("i.*, u.username AS owner_username, u.display_name AS owner_display_name, CASE WHEN p.id IS NULL THEN false ELSE true END AS purchased, CASE WHEN f.id IS NULL THEN false ELSE true END AS favorited, (SELECT COUNT(*) FROM inspiration_favorites fc WHERE fc.inspiration_id = i.id) AS favorite_count").
		Joins("JOIN users u ON u.id = i.owner_user_id").
		Joins("LEFT JOIN inspiration_purchases p ON p.inspiration_id = i.id AND p.buyer_user_id = ?", viewerUserID).
		Joins("LEFT JOIN inspiration_favorites f ON f.inspiration_id = i.id AND f.user_id = ?", viewerUserID).
		Where("i.status = ?", "published")
	if scope == "favorites" {
		query = query.Where("f.id IS NOT NULL")
	}
	if kind != "" && kind != "all" {
		query = query.Where("i.kind = ?", kind)
	}
	if keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("i.title LIKE ? OR i.description LIKE ? OR u.username LIKE ? OR u.display_name LIKE ?", pattern, pattern, pattern, pattern)
	}
	if limit <= 0 || limit > 100 {
		limit = 24
	}
	return rows, query.Order("i.created_at DESC").Limit(limit).Scan(&rows).Error
}

func (r *Repository) Inspiration(id string) (*model.Inspiration, error) {
	var item model.Inspiration
	if err := r.db.First(&item, "id = ? AND status = ?", id, "published").Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateInspiration(item *model.Inspiration) error {
	return r.db.Create(item).Error
}

func (r *Repository) InspirationPurchased(userID string, inspirationID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.InspirationPurchase{}).Where("buyer_user_id = ? AND inspiration_id = ?", userID, inspirationID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) SetInspirationFavorite(userID string, inspirationID string, favorite bool) error {
	if favorite {
		entry := model.InspirationFavorite{ID: newRepositoryID(), InspirationID: inspirationID, UserID: userID, CreatedAt: time.Now()}
		return r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "inspiration_id"}, {Name: "user_id"}}, DoNothing: true}).Create(&entry).Error
	}
	return r.db.Where("inspiration_id = ? AND user_id = ?", inspirationID, userID).Delete(&model.InspirationFavorite{}).Error
}

// PurchaseInspiration 在同一事务内完成买家扣款、作者入账和购买凭证创建。
func (r *Repository) PurchaseInspiration(item *model.Inspiration, buyerUserID string) (bool, error) {
	if item == nil || item.PriceMicrocredits <= 0 || item.OwnerUserID == buyerUserID {
		return false, nil
	}
	purchased := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		purchase := model.InspirationPurchase{
			ID: newRepositoryID(), InspirationID: item.ID, BuyerUserID: buyerUserID,
			SellerUserID: item.OwnerUserID, PriceMicrocredits: item.PriceMicrocredits, CreatedAt: time.Now(),
		}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "inspiration_id"}, {Name: "buyer_user_id"}}, DoNothing: true}).Create(&purchase)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return nil
		}
		for _, userID := range []string{buyerUserID, item.OwnerUserID} {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.CreditAccount{UserID: userID}).Error; err != nil {
				return err
			}
		}
		updated := tx.Model(&model.CreditAccount{}).
			Where("user_id = ? AND available_microcredits >= ?", buyerUserID, item.PriceMicrocredits).
			Updates(map[string]any{"available_microcredits": gorm.Expr("available_microcredits - ?", item.PriceMicrocredits), "version": gorm.Expr("version + 1"), "updated_at": time.Now()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrInsufficientCredits
		}
		if err := tx.Model(&model.CreditAccount{}).Where("user_id = ?", item.OwnerUserID).Updates(map[string]any{
			"available_microcredits": gorm.Expr("available_microcredits + ?", item.PriceMicrocredits), "version": gorm.Expr("version + 1"), "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		var buyerAccount, sellerAccount model.CreditAccount
		if err := tx.First(&buyerAccount, "user_id = ?", buyerUserID).Error; err != nil { return err }
		if err := tx.First(&sellerAccount, "user_id = ?", item.OwnerUserID).Error; err != nil { return err }
		buyerReference := "inspiration:buy:" + purchase.ID
		sellerReference := "inspiration:sell:" + purchase.ID
		entries := []model.CreditLedgerEntry{
			{ID: newRepositoryID(), UserID: buyerUserID, Type: model.CreditLedgerInspirationPurchase, AmountMicrocredits: -item.PriceMicrocredits, AvailableDeltaMicrocredits: -item.PriceMicrocredits, AvailableAfterMicrocredits: buyerAccount.AvailableMicrocredits, ReservedAfterMicrocredits: buyerAccount.ReservedMicrocredits, Scene: "inspiration", Note: "购买灵感：" + item.Title, ReferenceKey: &buyerReference},
			{ID: newRepositoryID(), UserID: item.OwnerUserID, Type: model.CreditLedgerInspirationSale, AmountMicrocredits: item.PriceMicrocredits, AvailableDeltaMicrocredits: item.PriceMicrocredits, AvailableAfterMicrocredits: sellerAccount.AvailableMicrocredits, ReservedAfterMicrocredits: sellerAccount.ReservedMicrocredits, Scene: "inspiration", Note: "灵感售出：" + item.Title, ReferenceKey: &sellerReference},
		}
		if err := tx.Create(&entries).Error; err != nil { return err }
		if err := tx.Model(&model.Inspiration{}).Where("id = ?", item.ID).UpdateColumn("copy_count", gorm.Expr("copy_count + 1")).Error; err != nil { return err }
		purchased = true
		return nil
	})
	return purchased, err
}

func (r *Repository) IncrementInspirationCopyCount(id string) error {
	return r.db.Model(&model.Inspiration{}).Where("id = ?", id).UpdateColumn("copy_count", gorm.Expr("copy_count + 1")).Error
}
