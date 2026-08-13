package repository

import (
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TeamRow struct {
	model.Team
	Role        string `json:"role" gorm:"column:member_role"`
	MemberCount int64  `json:"memberCount" gorm:"column:member_count"`
}

type TeamResourceRow struct {
	model.TeamResourceShare
	SharerName string `json:"sharerName" gorm:"column:sharer_name"`
}

type TeamMemberRow struct {
	model.User
	TeamRole string `json:"teamRole" gorm:"column:team_role"`
}

func (r *Repository) TeamsForUser(userID string) ([]TeamRow, error) {
	rows := make([]TeamRow, 0)
	err := r.db.Table("teams t").Select("t.*, tm.role AS member_role, (SELECT COUNT(*) FROM team_members mc WHERE mc.team_id = t.id) AS member_count").
		Joins("JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = ?", userID).Order("t.updated_at DESC").Scan(&rows).Error
	return rows, err
}

func (r *Repository) Team(id string) (*model.Team, error) {
	var item model.Team
	if err := r.db.First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) TeamByJoinCode(code string) (*model.Team, error) {
	var item model.Team
	if err := r.db.First(&item, "join_code = ?", code).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateTeam(team *model.Team, member *model.TeamMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(team).Error; err != nil {
			return err
		}
		return tx.Create(member).Error
	})
}

func (r *Repository) AddTeamMember(member *model.TeamMember) (bool, error) {
	result := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "team_id"}, {Name: "user_id"}}, DoNothing: true}).Create(member)
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) TeamMember(teamID, userID string) (*model.TeamMember, error) {
	var item model.TeamMember
	if err := r.db.First(&item, "team_id = ? AND user_id = ?", teamID, userID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) TeamMembers(teamID string) ([]TeamMemberRow, error) {
	rows := make([]TeamMemberRow, 0)
	err := r.db.Table("users u").Select("u.*, tm.role AS team_role").Joins("JOIN team_members tm ON tm.user_id = u.id").Where("tm.team_id = ?", teamID).Order("tm.created_at ASC").Scan(&rows).Error
	return rows, err
}

func (r *Repository) LeaveTeam(teamID, userID string) error {
	return r.db.Where("team_id = ? AND user_id = ?", teamID, userID).Delete(&model.TeamMember{}).Error
}

func (r *Repository) TeamResources(teamID, sourceType string) ([]TeamResourceRow, error) {
	rows := make([]TeamResourceRow, 0)
	query := r.db.Table("team_resource_shares trs").Select("trs.*, COALESCE(NULLIF(u.display_name, ''), u.username) AS sharer_name").Joins("JOIN users u ON u.id = trs.shared_by").Where("trs.team_id = ?", teamID)
	if sourceType != "" && sourceType != "all" {
		query = query.Where("trs.source_type = ?", sourceType)
	}
	return rows, query.Order("trs.created_at DESC").Scan(&rows).Error
}

func (r *Repository) SaveTeamResourceShare(item *model.TeamResourceShare) error {
	return r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "team_id"}, {Name: "source_type"}, {Name: "source_id"}}, DoUpdates: clause.Assignments(map[string]any{"shared_by": item.SharedBy, "title": item.Title, "preview_url": item.PreviewURL, "metadata_json": item.MetadataJSON, "updated_at": time.Now()})}).Create(item).Error
}

func (r *Repository) TeamResourceShare(teamID, sourceType, sourceID string) (*model.TeamResourceShare, error) {
	var item model.TeamResourceShare
	if err := r.db.First(&item, "team_id = ? AND source_type = ? AND source_id = ?", teamID, sourceType, sourceID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) DeleteTeamResourceShare(teamID, sourceType, sourceID, sharedBy string) error {
	return r.db.Where("team_id = ? AND source_type = ? AND source_id = ? AND shared_by = ?", teamID, sourceType, sourceID, sharedBy).Delete(&model.TeamResourceShare{}).Error
}
