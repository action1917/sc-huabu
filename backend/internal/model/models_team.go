package model

import "time"

// UserInviteCode 为每个用户提供稳定的邀请码；邀请码不承载权限，仅用于归因。
type UserInviteCode struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	UserID    string    `json:"userId" gorm:"uniqueIndex;size:36"`
	Code      string    `json:"code" gorm:"uniqueIndex;size:16"`
	CreatedAt time.Time `json:"createdAt"`
}

type InviteReferral struct {
	ID                        string    `json:"id" gorm:"primaryKey;size:36"`
	InviterUserID             string    `json:"inviterUserId" gorm:"index;size:36"`
	InviteeUserID             string    `json:"inviteeUserId" gorm:"uniqueIndex;size:36"`
	InviteCode                string    `json:"inviteCode" gorm:"index;size:16"`
	InviterRewardMicrocredits int64     `json:"inviterRewardMicrocredits"`
	InviteeRewardMicrocredits int64     `json:"inviteeRewardMicrocredits"`
	CreatedAt                 time.Time `json:"createdAt" gorm:"index"`
}

type Team struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	OwnerUserID string    `json:"ownerUserId" gorm:"index;size:36"`
	Name        string    `json:"name" gorm:"size:80;index"`
	Description string    `json:"description" gorm:"size:500"`
	JoinCode    string    `json:"joinCode" gorm:"uniqueIndex;size:20"`
	CreatedAt   time.Time `json:"createdAt" gorm:"index"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type TeamMember struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	TeamID    string    `json:"teamId" gorm:"uniqueIndex:idx_team_member,priority:1;index;size:36"`
	UserID    string    `json:"userId" gorm:"uniqueIndex:idx_team_member,priority:2;index;size:36"`
	Role      string    `json:"role" gorm:"size:20"`
	CreatedAt time.Time `json:"createdAt" gorm:"index"`
}

// TeamResourceShare 只保存用户明确公开到团队的资源引用，撤销后团队立即失去入口。
type TeamResourceShare struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	TeamID       string    `json:"teamId" gorm:"uniqueIndex:idx_team_share_source,priority:1;index;size:36"`
	SourceType   string    `json:"sourceType" gorm:"uniqueIndex:idx_team_share_source,priority:2;index;size:24"`
	SourceID     string    `json:"sourceId" gorm:"uniqueIndex:idx_team_share_source,priority:3;index;size:80"`
	SharedBy     string    `json:"sharedBy" gorm:"index;size:36"`
	Title        string    `json:"title" gorm:"size:160"`
	PreviewURL   string    `json:"previewUrl" gorm:"type:text"`
	MetadataJSON string    `json:"-" gorm:"type:text"`
	CreatedAt    time.Time `json:"createdAt" gorm:"index"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SkillPurchase struct {
	ID                string    `json:"id" gorm:"primaryKey;size:36"`
	SkillID           string    `json:"skillId" gorm:"uniqueIndex:idx_skill_buyer,priority:1;size:36"`
	BuyerUserID       string    `json:"buyerUserId" gorm:"uniqueIndex:idx_skill_buyer,priority:2;index;size:36"`
	SellerUserID      string    `json:"sellerUserId" gorm:"index;size:36"`
	PriceMicrocredits int64     `json:"priceMicrocredits"`
	CreatedAt         time.Time `json:"createdAt" gorm:"index"`
}
