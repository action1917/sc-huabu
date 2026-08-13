package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) InviteCodeForUser(userID string) (*model.UserInviteCode, error) {
	var item model.UserInviteCode
	if err := r.db.First(&item, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) InviteCode(code string) (*model.UserInviteCode, error) {
	var item model.UserInviteCode
	if err := r.db.First(&item, "code = ?", code).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateInviteCode(item *model.UserInviteCode) error {
	return r.db.Create(item).Error
}

func (r *Repository) InviteReferralForInvitee(userID string) (*model.InviteReferral, error) {
	var item model.InviteReferral
	if err := r.db.First(&item, "invitee_user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) InviteReferralCount(userID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.InviteReferral{}).Where("inviter_user_id = ?", userID).Count(&count).Error
	return count, err
}

// ApplyInviteReward 以 invitee 唯一索引保证奖励只发放一次，并同步写入双方积分流水。
func (r *Repository) ApplyInviteReward(inviterID, inviteeID, code string, inviterReward, inviteeReward int64) (bool, error) {
	if inviterID == "" || inviteeID == "" || inviterID == inviteeID {
		return false, errors.New("邀请关系无效")
	}
	applied := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		referral := model.InviteReferral{ID: newRepositoryID(), InviterUserID: inviterID, InviteeUserID: inviteeID, InviteCode: code, InviterRewardMicrocredits: inviterReward, InviteeRewardMicrocredits: inviteeReward, CreatedAt: time.Now()}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "invitee_user_id"}}, DoNothing: true}).Create(&referral)
		if created.Error != nil || created.RowsAffected == 0 {
			return created.Error
		}
		for _, userID := range []string{inviterID, inviteeID} {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.CreditAccount{UserID: userID}).Error; err != nil {
				return err
			}
		}
		grants := []struct {
			userID          string
			amount          int64
			note, reference string
		}{
			{inviterID, inviterReward, "邀请新用户奖励", "invite:inviter:" + referral.ID},
			{inviteeID, inviteeReward, "受邀注册奖励", "invite:invitee:" + referral.ID},
		}
		for _, grant := range grants {
			if grant.amount <= 0 {
				continue
			}
			if err := tx.Model(&model.CreditAccount{}).Where("user_id = ?", grant.userID).Updates(map[string]any{"available_microcredits": gorm.Expr("available_microcredits + ?", grant.amount), "version": gorm.Expr("version + 1"), "updated_at": time.Now()}).Error; err != nil {
				return err
			}
			var account model.CreditAccount
			if err := tx.First(&account, "user_id = ?", grant.userID).Error; err != nil {
				return err
			}
			reference := grant.reference
			entry := model.CreditLedgerEntry{ID: newRepositoryID(), UserID: grant.userID, Type: model.CreditLedgerInviteReward, AmountMicrocredits: grant.amount, AvailableDeltaMicrocredits: grant.amount, AvailableAfterMicrocredits: account.AvailableMicrocredits, ReservedAfterMicrocredits: account.ReservedMicrocredits, Scene: "invite", Note: grant.note, ReferenceKey: &reference, CreatedAt: time.Now()}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
		}
		applied = true
		return nil
	})
	return applied, err
}
