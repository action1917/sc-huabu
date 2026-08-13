package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) SkillPurchased(userID, skillID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.SkillPurchase{}).Where("buyer_user_id = ? AND skill_id = ?", userID, skillID).Count(&count).Error
	return count > 0, err
}

// PurchaseSkill 与灵感购买保持同一账务合同：购买凭证、扣款、作者入账和双方流水同事务提交。
func (r *Repository) PurchaseSkill(skill *model.Skill, buyerUserID string) (bool, error) {
	if skill == nil || skill.PriceMicrocredits <= 0 || skill.OwnerID == buyerUserID {
		return false, nil
	}
	purchased := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		purchase := model.SkillPurchase{ID: newRepositoryID(), SkillID: skill.ID, BuyerUserID: buyerUserID, SellerUserID: skill.OwnerID, PriceMicrocredits: skill.PriceMicrocredits, CreatedAt: time.Now()}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "skill_id"}, {Name: "buyer_user_id"}}, DoNothing: true}).Create(&purchase)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return nil
		}
		for _, uid := range []string{buyerUserID, skill.OwnerID} {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.CreditAccount{UserID: uid}).Error; err != nil {
				return err
			}
		}
		updated := tx.Model(&model.CreditAccount{}).Where("user_id = ? AND available_microcredits >= ?", buyerUserID, skill.PriceMicrocredits).Updates(map[string]any{"available_microcredits": gorm.Expr("available_microcredits - ?", skill.PriceMicrocredits), "version": gorm.Expr("version + 1"), "updated_at": time.Now()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrInsufficientCredits
		}
		if err := tx.Model(&model.CreditAccount{}).Where("user_id = ?", skill.OwnerID).Updates(map[string]any{"available_microcredits": gorm.Expr("available_microcredits + ?", skill.PriceMicrocredits), "version": gorm.Expr("version + 1"), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		var buyer, seller model.CreditAccount
		if err := tx.First(&buyer, "user_id = ?", buyerUserID).Error; err != nil {
			return err
		}
		if err := tx.First(&seller, "user_id = ?", skill.OwnerID).Error; err != nil {
			return err
		}
		buyRef, sellRef := "skill:buy:"+purchase.ID, "skill:sell:"+purchase.ID
		entries := []model.CreditLedgerEntry{{ID: newRepositoryID(), UserID: buyerUserID, Type: model.CreditLedgerSkillPurchase, AmountMicrocredits: -skill.PriceMicrocredits, AvailableDeltaMicrocredits: -skill.PriceMicrocredits, AvailableAfterMicrocredits: buyer.AvailableMicrocredits, ReservedAfterMicrocredits: buyer.ReservedMicrocredits, Scene: "skill", Note: "购买技能：" + skill.Name, ReferenceKey: &buyRef, CreatedAt: time.Now()}, {ID: newRepositoryID(), UserID: skill.OwnerID, Type: model.CreditLedgerSkillSale, AmountMicrocredits: skill.PriceMicrocredits, AvailableDeltaMicrocredits: skill.PriceMicrocredits, AvailableAfterMicrocredits: seller.AvailableMicrocredits, ReservedAfterMicrocredits: seller.ReservedMicrocredits, Scene: "skill", Note: "技能售出：" + skill.Name, ReferenceKey: &sellRef, CreatedAt: time.Now()}}
		if err := tx.Create(&entries).Error; err != nil {
			return err
		}
		purchased = true
		return nil
	})
	return purchased, err
}

type SkillListFilter struct {
	UserID string
	Scope  string
	Search string
	Tag    string
	Sort   string
	Limit  int
	Offset int
}

type SkillMetrics struct {
	SkillID    string
	AddedCount int64
	LikeCount  int64
}

func (r *Repository) Skills(filter SkillListFilter) ([]model.Skill, int64, error) {
	query := r.db.Model(&model.Skill{}).Where("skills.status = ?", 1)
	switch filter.Scope {
	case "mine":
		query = query.Joins("LEFT JOIN user_skill_states ON user_skill_states.skill_id = skills.id AND user_skill_states.user_id = ?", filter.UserID).
			Where("skills.owner_id = ? OR (user_skill_states.added = ? AND skills.is_private = ?)", filter.UserID, true, false)
	case "created":
		query = query.Where("skills.owner_id = ?", filter.UserID)
	case "favorites":
		query = query.Joins("JOIN user_skill_states ON user_skill_states.skill_id = skills.id AND user_skill_states.user_id = ? AND user_skill_states.liked = ?", filter.UserID, true).
			Where("skills.owner_id = ? OR skills.is_private = ?", filter.UserID, false)
	default:
		query = query.Where("skills.is_private = ?", false)
	}
	if filter.Search != "" {
		pattern := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Joins("LEFT JOIN users skill_owners ON skill_owners.id = skills.owner_id").
			Where("lower(skills.name) LIKE ? OR lower(skills.description) LIKE ? OR lower(skills.author_name) LIKE ? OR lower(skill_owners.display_name) LIKE ? OR lower(skill_owners.username) LIKE ?", pattern, pattern, pattern, pattern, pattern)
	}
	if filter.Tag != "" {
		query = query.Where("skills.tag = ?", filter.Tag)
	}
	// 用户状态按用户和技能唯一，列表 JOIN 不会重复；不要设置 DISTINCT，避免 GORM 将其带入 PostgreSQL 的热门排序查询。
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "skills.sort_weight desc, skills.updated_at desc"
	switch filter.Sort {
	case "new":
		order = "skills.created_at desc"
	case "popular":
		order = "(skills.initial_added_count + (SELECT COUNT(*) FROM user_skill_states metric_states WHERE metric_states.skill_id = skills.id AND metric_states.added = true)) desc, skills.updated_at desc"
	}
	var skills []model.Skill
	err := query.Select("skills.*").Order(order).Limit(filter.Limit).Offset(filter.Offset).Find(&skills).Error
	return skills, total, err
}

// 内置技能使用稳定的外部技能 ID 幂等更新，用户加入和收藏关系保留在独立状态表中。
func (r *Repository) UpsertBuiltinSkills(skills []model.Skill) error {
	if len(skills) == 0 {
		return errors.New("builtin skills are empty")
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"owner_id", "author_name", "author_avatar_url", "name", "description", "instruction", "status", "source", "tag", "sort_weight", "is_private",
			"markdown_url", "showcase_media_json", "extra_info", "initial_like_count", "initial_added_count", "created_at", "updated_at",
		}),
	}).Create(&skills).Error
}

func (r *Repository) Skill(id string) (*model.Skill, error) {
	var skill model.Skill
	if err := r.db.First(&skill, "id = ? AND status = ?", id, 1).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *Repository) CreateSkill(skill *model.Skill, ownerState *model.UserSkillState) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(skill).Error; err != nil {
			return err
		}
		return tx.Create(ownerState).Error
	})
}

func (r *Repository) DeleteSkill(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.UserSkillState{}, "skill_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Skill{}, "id = ?", id).Error
	})
}

func (r *Repository) UserSkillState(userID string, skillID string) (*model.UserSkillState, error) {
	var state model.UserSkillState
	if err := r.db.First(&state, "user_id = ? AND skill_id = ?", userID, skillID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (r *Repository) UserSkillStatesBySkillIDs(userID string, skillIDs []string) ([]model.UserSkillState, error) {
	if len(skillIDs) == 0 {
		return nil, nil
	}
	var states []model.UserSkillState
	err := r.db.Find(&states, "user_id = ? AND skill_id IN ?", userID, skillIDs).Error
	return states, err
}

func (r *Repository) SetUserSkillAdded(state *model.UserSkillState) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "skill_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"added", "updated_at"}),
	}).Create(state).Error
}

func (r *Repository) SetUserSkillLiked(state *model.UserSkillState) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "skill_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"liked", "updated_at"}),
	}).Create(state).Error
}

func (r *Repository) SkillMetrics(skillIDs []string) (map[string]SkillMetrics, error) {
	metrics := make(map[string]SkillMetrics, len(skillIDs))
	if len(skillIDs) == 0 {
		return metrics, nil
	}
	var rows []SkillMetrics
	err := r.db.Model(&model.UserSkillState{}).
		Select("skill_id, SUM(CASE WHEN added THEN 1 ELSE 0 END) AS added_count, SUM(CASE WHEN liked THEN 1 ELSE 0 END) AS like_count").
		Where("skill_id IN ?", skillIDs).
		Group("skill_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		metrics[item.SkillID] = item
	}
	return metrics, nil
}

func (r *Repository) SkillOwners(ownerIDs []string) (map[string]model.User, error) {
	owners := make(map[string]model.User, len(ownerIDs))
	if len(ownerIDs) == 0 {
		return owners, nil
	}
	var users []model.User
	if err := r.db.Find(&users, "id IN ?", ownerIDs).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		owners[user.ID] = user
	}
	return owners, nil
}

func (r *Repository) SkillOwnerAvatars(ownerIDs []string) (map[string]string, error) {
	avatars := make(map[string]string, len(ownerIDs))
	if len(ownerIDs) == 0 {
		return avatars, nil
	}
	var identities []model.UserIdentity
	if err := r.db.Where("user_id IN ? AND avatar_url <> ''", ownerIDs).Order("updated_at desc").Find(&identities).Error; err != nil {
		return nil, err
	}
	for _, identity := range identities {
		if avatars[identity.UserID] == "" {
			avatars[identity.UserID] = identity.AvatarURL
		}
	}
	return avatars, nil
}
