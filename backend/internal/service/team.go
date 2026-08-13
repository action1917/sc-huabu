package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type ShareTeamResourceRequest struct {
	SourceType string         `json:"sourceType"`
	SourceID   string         `json:"sourceId"`
	Title      string         `json:"title"`
	PreviewURL string         `json:"previewUrl"`
	Metadata   map[string]any `json:"metadata"`
}
type TeamDetail struct {
	Team    repository.TeamRow `json:"team"`
	Members []TeamMemberItem   `json:"members"`
}
type TeamMemberItem struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Role        string `json:"role"`
}

func (s *Service) Teams(user *model.User) ([]repository.TeamRow, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	return s.repo.TeamsForUser(user.ID)
}

func (s *Service) CreateTeam(user *model.User, req CreateTeamRequest) (*model.Team, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, BadAuthRequest("请输入团队名称")
	}
	now := time.Now()
	team := &model.Team{ID: newID(), OwnerUserID: user.ID, Name: truncateRunes(name, 80), Description: truncateRunes(strings.TrimSpace(req.Description), 500), JoinCode: strings.ToUpper(randomToken()[:10]), CreatedAt: now, UpdatedAt: now}
	member := &model.TeamMember{ID: newID(), TeamID: team.ID, UserID: user.ID, Role: "owner", CreatedAt: now}
	if err := s.repo.CreateTeam(team, member); err != nil {
		return nil, err
	}
	return team, nil
}

func (s *Service) JoinTeam(user *model.User, rawCode string) (*model.Team, bool, error) {
	if user == nil {
		return nil, false, Unauthorized("请先登录")
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	team, err := s.repo.TeamByJoinCode(code)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, BadAuthRequest("团队邀请码不存在")
	}
	if err != nil {
		return nil, false, err
	}
	joined, err := s.repo.AddTeamMember(&model.TeamMember{ID: newID(), TeamID: team.ID, UserID: user.ID, Role: "member", CreatedAt: time.Now()})
	return team, joined, err
}

func (s *Service) TeamDetail(user *model.User, id string) (*TeamDetail, error) {
	if err := s.requireTeamMember(user, id); err != nil {
		return nil, err
	}
	teams, err := s.repo.TeamsForUser(user.ID)
	if err != nil {
		return nil, err
	}
	var team repository.TeamRow
	for _, row := range teams {
		if row.ID == id {
			team = row
			break
		}
	}
	if team.ID == "" {
		return nil, Forbidden("你不是该团队成员")
	}
	users, err := s.repo.TeamMembers(id)
	if err != nil {
		return nil, err
	}
	members := make([]TeamMemberItem, 0, len(users))
	for i := range users {
		public, publicErr := s.PublicAuthUser(&users[i].User)
		if publicErr == nil {
			members = append(members, TeamMemberItem{ID: public.ID, Username: public.Username, DisplayName: public.DisplayName, AvatarURL: public.AvatarURL, Role: users[i].TeamRole})
		}
	}
	return &TeamDetail{Team: team, Members: members}, nil
}

func (s *Service) LeaveTeam(user *model.User, id string) error {
	team, err := s.repo.Team(id)
	if err != nil {
		return err
	}
	if team.OwnerUserID == user.ID {
		return BadAuthRequest("团队创建者不能直接退出，请先转让或解散团队")
	}
	if err := s.requireTeamMember(user, id); err != nil {
		return err
	}
	return s.repo.LeaveTeam(id, user.ID)
}

func (s *Service) TeamResources(user *model.User, teamID, sourceType string) ([]repository.TeamResourceRow, error) {
	if err := s.requireTeamMember(user, teamID); err != nil {
		return nil, err
	}
	rows, err := s.repo.TeamResources(teamID, strings.ToLower(strings.TrimSpace(sourceType)))
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].SourceType == "resource" {
			if resource, e := s.repo.Resource(rows[i].SourceID); e == nil {
				if u, e := s.directResourceURL(resource, time.Now().Add(4*time.Hour)); e == nil {
					rows[i].PreviewURL = u
				}
			}
		}
	}
	return rows, nil
}

func (s *Service) ShareTeamResource(user *model.User, teamID string, req ShareTeamResourceRequest) (*model.TeamResourceShare, error) {
	if err := s.requireTeamMember(user, teamID); err != nil {
		return nil, err
	}
	sourceType := strings.ToLower(strings.TrimSpace(req.SourceType))
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return nil, BadAuthRequest("请选择要共享的资源")
	}
	title, preview, err := s.validateOwnedTeamSource(user.ID, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Title) != "" {
		title = truncateRunes(strings.TrimSpace(req.Title), 160)
	}
	metadata, _ := json.Marshal(req.Metadata)
	now := time.Now()
	item := &model.TeamResourceShare{ID: newID(), TeamID: teamID, SourceType: sourceType, SourceID: sourceID, SharedBy: user.ID, Title: title, PreviewURL: preview, MetadataJSON: string(metadata), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.SaveTeamResourceShare(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UnshareTeamResource(user *model.User, teamID, sourceType, sourceID string) error {
	if err := s.requireTeamMember(user, teamID); err != nil {
		return err
	}
	return s.repo.DeleteTeamResourceShare(teamID, sourceType, sourceID, user.ID)
}

func (s *Service) TeamResourceContent(user *model.User, teamID, sourceType, sourceID string) (map[string]any, error) {
	if err := s.requireTeamMember(user, teamID); err != nil {
		return nil, err
	}
	share, err := s.repo.TeamResourceShare(teamID, sourceType, sourceID)
	if err != nil {
		return nil, BadAuthRequest("团队共享内容不存在或已撤销")
	}
	switch share.SourceType {
	case "skill":
		skill, e := s.repo.Skill(share.SourceID)
		if e != nil {
			return nil, e
		}
		return map[string]any{"type": "skill", "id": skill.ID, "title": skill.Name, "instruction": skill.Instruction, "description": skill.Description}, nil
	case "inspiration":
		item, e := s.repo.Inspiration(share.SourceID)
		if e != nil {
			return nil, e
		}
		params := map[string]any{}
		_ = json.Unmarshal([]byte(item.ParametersJSON), &params)
		return map[string]any{"type": "inspiration", "id": item.ID, "title": item.Title, "prompt": item.Prompt, "parameters": params}, nil
	case "resource":
		resource, e := s.repo.Resource(share.SourceID)
		if e != nil {
			return nil, e
		}
		url, e := s.directResourceURL(resource, time.Now().Add(4*time.Hour))
		if e != nil {
			return nil, e
		}
		return map[string]any{"type": "resource", "id": resource.ID, "url": url, "mimeType": resource.MimeType}, nil
	case "asset", "work":
		asset, e := s.repo.AssetForUser(share.SharedBy, share.SourceID)
		if e != nil {
			return nil, e
		}
		var payload any
		_ = json.Unmarshal([]byte(asset.PayloadJSON), &payload)
		return map[string]any{"type": share.SourceType, "id": asset.ID, "title": asset.Title, "kind": asset.Kind, "category": asset.Category, "payload": payload}, nil
	case "project":
		project, e := s.repo.ProjectForUser(share.SharedBy, share.SourceID)
		if e != nil {
			return nil, e
		}
		return map[string]any{"type": "project", "id": project.ID, "title": project.Name, "description": project.Description, "aspectRatio": project.AspectRatio, "styleProfile": project.StyleProfileJSON}, nil
	default:
		return map[string]any{"type": share.SourceType, "id": share.SourceID, "title": share.Title}, nil
	}
}

func (s *Service) requireTeamMember(user *model.User, teamID string) error {
	if user == nil {
		return Unauthorized("请先登录")
	}
	if _, err := s.repo.TeamMember(strings.TrimSpace(teamID), user.ID); errors.Is(err, gorm.ErrRecordNotFound) {
		return Forbidden("你不是该团队成员")
	} else {
		return err
	}
}

func (s *Service) validateOwnedTeamSource(userID, sourceType, sourceID string) (string, string, error) {
	switch sourceType {
	case "resource":
		r, e := s.repo.ResourceForUser(userID, sourceID)
		if e != nil {
			return "", "", BadAuthRequest("资源不存在或不属于当前用户")
		}
		return firstNonEmpty(r.ObjectKey, "资源"), "", nil
	case "asset", "work":
		a, e := s.repo.AssetForUser(userID, sourceID)
		if e != nil {
			return "", "", BadAuthRequest("作品或素材不存在")
		}
		return a.Title, "", nil
	case "project":
		p, e := s.repo.ProjectForUser(userID, sourceID)
		if e != nil {
			return "", "", BadAuthRequest("短剧项目不存在")
		}
		return p.Name, "", nil
	case "inspiration":
		i, e := s.repo.Inspiration(sourceID)
		if e != nil {
			return "", "", BadAuthRequest("灵感不存在")
		}
		if i.OwnerUserID != userID {
			paid, e := s.repo.InspirationPurchased(userID, sourceID)
			if e != nil || !paid {
				return "", "", Forbidden("仅作者或已解锁用户可共享该灵感")
			}
		}
		return i.Title, i.PreviewURL, nil
	case "skill":
		sk, e := s.repo.Skill(sourceID)
		if e != nil {
			return "", "", BadAuthRequest("技能不存在")
		}
		if sk.OwnerID != userID {
			paid, e := s.repo.SkillPurchased(userID, sourceID)
			if e != nil || (!paid && sk.PriceMicrocredits > 0) {
				return "", "", Forbidden("仅作者或已解锁用户可共享该技能")
			}
		}
		return sk.Name, "", nil
	default:
		return "", "", BadAuthRequest("暂不支持该资源类型")
	}
}
