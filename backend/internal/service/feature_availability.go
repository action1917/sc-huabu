package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const featureAvailabilitySettingKey = "feature_availability"

const (
	FeatureCreation     = "creation"
	FeatureShortDrama   = "shortDrama"
	FeatureInspirations = "inspirations"
	FeatureCanvas       = "canvas"
	FeatureTaskCenter   = "taskCenter"
	FeatureAssets       = "assets"
	FeatureSkills       = "skills"
	FeatureTeams        = "teams"
	FeatureCredits      = "credits"
	FeatureSettings     = "settings"
)

type FeatureAvailability struct {
	CreationEnabled     bool `json:"creationEnabled"`
	ShortDramaEnabled   bool `json:"shortDramaEnabled"`
	InspirationsEnabled bool `json:"inspirationsEnabled"`
	CanvasEnabled       bool `json:"canvasEnabled"`
	TaskCenterEnabled   bool `json:"taskCenterEnabled"`
	AssetsEnabled       bool `json:"assetsEnabled"`
	SkillsEnabled       bool `json:"skillsEnabled"`
	TeamsEnabled        bool `json:"teamsEnabled"`
	CreditsEnabled      bool `json:"creditsEnabled"`
	SettingsEnabled     bool `json:"settingsEnabled"`
}

type PublicFeatureAvailability struct {
	FeatureAvailability
	Configured bool      `json:"configured"`
	UpdatedBy  string    `json:"updatedBy,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}

func defaultFeatureAvailability() FeatureAvailability {
	// 缺少配置代表尚未由运维接管，默认保持现有功能全部开放。
	return FeatureAvailability{
		CreationEnabled: true, ShortDramaEnabled: true, InspirationsEnabled: true,
		CanvasEnabled: true, TaskCenterEnabled: true, AssetsEnabled: true,
		SkillsEnabled: true, TeamsEnabled: true, CreditsEnabled: true, SettingsEnabled: true,
	}
}

func (s *Service) FeatureAvailability() (*PublicFeatureAvailability, error) {
	setting, value, err := s.readFeatureAvailability()
	if err != nil {
		return nil, err
	}
	return publicFeatureAvailability(setting, value), nil
}

func (s *Service) AdminFeatureAvailability(actor *model.User) (*PublicFeatureAvailability, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	return s.FeatureAvailability()
}

func (s *Service) UpdateFeatureAvailability(actor *model.User, value FeatureAvailability) (*PublicFeatureAvailability, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	current, before, err := s.readFeatureAvailability()
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	setting := model.SystemSetting{Key: featureAvailabilitySettingKey, ValueJSON: string(encoded), UpdatedBy: actor.ID}
	if current != nil {
		setting.CreatedAt = current.CreatedAt
	}
	if err := s.repo.SaveSystemSetting(&setting); err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "feature_availability.update", "system_setting", featureAvailabilitySettingKey, "更新功能开放配置", map[string]any{"before": before, "after": value}); err != nil {
		return nil, err
	}
	return publicFeatureAvailability(&setting, value), nil
}

func (s *Service) FeatureEnabled(feature string) (bool, error) {
	_, value, err := s.readFeatureAvailability()
	if err != nil {
		return false, err
	}
	switch feature {
	case FeatureCreation:
		return value.CreationEnabled, nil
	case FeatureShortDrama:
		return value.ShortDramaEnabled, nil
	case FeatureInspirations:
		return value.InspirationsEnabled, nil
	case FeatureCanvas:
		return value.CanvasEnabled, nil
	case FeatureTaskCenter:
		return value.TaskCenterEnabled, nil
	case FeatureAssets:
		return value.AssetsEnabled, nil
	case FeatureSkills:
		return value.SkillsEnabled, nil
	case FeatureTeams:
		return value.TeamsEnabled, nil
	case FeatureCredits:
		return value.CreditsEnabled, nil
	case FeatureSettings:
		return value.SettingsEnabled, nil
	default:
		return false, errors.New("未知功能开放配置")
	}
}

func (s *Service) RequireFeature(feature string) error {
	enabled, err := s.FeatureEnabled(feature)
	if err != nil {
		return err
	}
	if enabled {
		return nil
	}
	switch feature {
	case FeatureCreation:
		return Forbidden("创作功能暂未开放")
	case FeatureShortDrama:
		return Forbidden("短剧创作暂未开放")
	case FeatureInspirations:
		return Forbidden("灵感广场暂未开放")
	case FeatureCanvas:
		return Forbidden("画布功能暂未开放")
	case FeatureTaskCenter:
		return Forbidden("任务中心暂未开放")
	case FeatureAssets:
		return Forbidden("资产功能暂未开放")
	case FeatureSkills:
		return Forbidden("技能库暂未开放")
	case FeatureTeams:
		return Forbidden("团队空间暂未开放")
	case FeatureCredits:
		return Forbidden("积分功能暂未开放")
	case FeatureSettings:
		return Forbidden("设置功能暂未开放")
	default:
		return Forbidden("该功能暂未开放")
	}
}

func (s *Service) readFeatureAvailability() (*model.SystemSetting, FeatureAvailability, error) {
	setting, err := s.repo.SystemSetting(featureAvailabilitySettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, defaultFeatureAvailability(), nil
	}
	if err != nil {
		return nil, FeatureAvailability{}, err
	}
	// 旧版本只保存了部分字段；以全开默认值为底合并，升级后不会误关新增加的功能。
	value := defaultFeatureAvailability()
	if strings.TrimSpace(setting.ValueJSON) == "" || json.Unmarshal([]byte(setting.ValueJSON), &value) != nil {
		return nil, FeatureAvailability{}, errors.New("功能开放配置格式无效")
	}
	return setting, value, nil
}

func publicFeatureAvailability(setting *model.SystemSetting, value FeatureAvailability) *PublicFeatureAvailability {
	result := &PublicFeatureAvailability{FeatureAvailability: value, Configured: setting != nil}
	if setting != nil {
		result.UpdatedBy = setting.UpdatedBy
		result.UpdatedAt = setting.UpdatedAt
	}
	return result
}
