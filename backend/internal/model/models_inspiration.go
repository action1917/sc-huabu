package model

import "time"

// Inspiration 保存公开展示所需信息；收费内容的提示词和参数只通过复刻接口返回。
type Inspiration struct {
	ID                string    `json:"id" gorm:"primaryKey;size:36"`
	OwnerUserID       string    `json:"ownerUserId" gorm:"index;size:36"`
	Kind              string    `json:"kind" gorm:"index;size:24"`
	Title             string    `json:"title" gorm:"size:160"`
	Description       string    `json:"description" gorm:"size:500"`
	PreviewURL        string    `json:"previewUrl" gorm:"type:text"`
	PreviewResourceID string    `json:"-" gorm:"index;size:36"`
	Prompt            string    `json:"prompt,omitempty" gorm:"type:text"`
	ParametersJSON    string    `json:"parametersJson,omitempty" gorm:"type:text"`
	SourceType        string    `json:"sourceType" gorm:"size:32"`
	SourceID          string    `json:"sourceId" gorm:"index;size:80"`
	PriceMicrocredits int64     `json:"priceMicrocredits"`
	CopyCount         int64     `json:"copyCount"`
	Status            string    `json:"status" gorm:"index;size:24"`
	CreatedAt         time.Time `json:"createdAt" gorm:"index"`
	UpdatedAt         time.Time `json:"updatedAt" gorm:"index"`
}

type InspirationPurchase struct {
	ID                string    `json:"id" gorm:"primaryKey;size:36"`
	InspirationID     string    `json:"inspirationId" gorm:"uniqueIndex:idx_inspiration_buyer,priority:1;size:36"`
	BuyerUserID       string    `json:"buyerUserId" gorm:"uniqueIndex:idx_inspiration_buyer,priority:2;index;size:36"`
	SellerUserID      string    `json:"sellerUserId" gorm:"index;size:36"`
	PriceMicrocredits int64     `json:"priceMicrocredits"`
	CreatedAt         time.Time `json:"createdAt" gorm:"index"`
}

// InspirationFavorite 保存用户对灵感广场内容的收藏关系。
type InspirationFavorite struct {
	ID            string    `json:"id" gorm:"primaryKey;size:36"`
	InspirationID string    `json:"inspirationId" gorm:"uniqueIndex:idx_inspiration_favorite_user,priority:1;index;size:36"`
	UserID        string    `json:"userId" gorm:"uniqueIndex:idx_inspiration_favorite_user,priority:2;index;size:36"`
	CreatedAt     time.Time `json:"createdAt" gorm:"index"`
}
