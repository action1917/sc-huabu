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

const maxInspirationPrice = int64(100_000) * CreditScale

type CreateInspirationRequest struct {
	Kind              string         `json:"kind"`
	Title             string         `json:"title"`
	Description       string         `json:"description"`
	PreviewURL        string         `json:"previewUrl"`
	PreviewResourceID string         `json:"previewResourceId"`
	Prompt            string         `json:"prompt"`
	Parameters        map[string]any `json:"parameters"`
	SourceType        string         `json:"sourceType"`
	SourceID          string         `json:"sourceId"`
	PriceMicrocredits int64          `json:"priceMicrocredits"`
}

type InspirationListItem struct {
	ID                string         `json:"id"`
	Kind              string         `json:"kind"`
	Title             string         `json:"title"`
	Description       string         `json:"description"`
	PreviewURL        string         `json:"previewUrl"`
	PromptPreview     string         `json:"promptPreview,omitempty"`
	ParametersPreview map[string]any `json:"parametersPreview,omitempty"`
	SourceType        string         `json:"sourceType"`
	PriceMicrocredits int64          `json:"priceMicrocredits"`
	CopyCount         int64          `json:"copyCount"`
	Owner             bool           `json:"owner"`
	Unlocked          bool           `json:"unlocked"`
	Favorite          bool           `json:"favorite"`
	FavoriteCount     int64          `json:"favoriteCount"`
	AuthorName        string         `json:"authorName"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type InspirationReplica struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Prompt     string         `json:"prompt"`
	Parameters map[string]any `json:"parameters"`
	SourceType string         `json:"sourceType"`
	SourceID   string         `json:"sourceId"`
	Purchased  bool           `json:"purchased"`
}

func (s *Service) ListInspirations(user *model.User, scope string, kind string, keyword string, limit int) ([]InspirationListItem, error) {
	if user == nil { return nil, Unauthorized("请先登录") }
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope != "favorites" { scope = "public" }
	rows, err := s.repo.Inspirations(user.ID, scope, strings.ToLower(strings.TrimSpace(kind)), truncateRunes(strings.TrimSpace(keyword), 80), limit)
	if err != nil { return nil, err }
	items := make([]InspirationListItem, 0, len(rows))
	for _, row := range rows {
		owner := row.OwnerUserID == user.ID
		unlocked := row.PriceMicrocredits == 0 || owner || row.Purchased
		previewURL := row.PreviewURL
		if row.PreviewResourceID != "" {
			if resource, resourceErr := s.repo.Resource(row.PreviewResourceID); resourceErr == nil && resource.UserID == row.OwnerUserID {
				if signedURL, signedErr := s.directResourceURL(resource, time.Now().Add(24*time.Hour)); signedErr == nil { previewURL = signedURL }
			}
		}
		item := InspirationListItem{ID: row.ID, Kind: row.Kind, Title: row.Title, Description: row.Description, PreviewURL: previewURL, SourceType: row.SourceType, PriceMicrocredits: row.PriceMicrocredits, CopyCount: row.CopyCount, Owner: owner, Unlocked: unlocked, Favorite: row.Favorited, FavoriteCount: row.FavoriteCount, AuthorName: firstNonEmpty(row.OwnerDisplayName, row.OwnerUsername), CreatedAt: row.CreatedAt}
		if unlocked {
			item.PromptPreview = truncateRunes(row.Prompt, 160)
			_ = json.Unmarshal([]byte(row.ParametersJSON), &item.ParametersPreview)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) SetInspirationFavorite(user *model.User, id string, favorite bool) error {
	if user == nil { return Unauthorized("请先登录") }
	item, err := s.repo.Inspiration(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) { return &AuthError{Status: 404, Message: "灵感不存在或已下架"} }
	if err != nil { return err }
	return s.repo.SetInspirationFavorite(user.ID, item.ID, favorite)
}

// UnlockInspiration 是收费内容唯一允许扣积分的入口，必须由用户确认后单独调用。
func (s *Service) UnlockInspiration(user *model.User, id string) (bool, error) {
	if user == nil { return false, Unauthorized("请先登录") }
	item, err := s.repo.Inspiration(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) { return false, &AuthError{Status: 404, Message: "灵感不存在或已下架"} }
	if err != nil { return false, err }
	if item.PriceMicrocredits <= 0 || item.OwnerUserID == user.ID { return false, nil }
	alreadyPurchased, err := s.repo.InspirationPurchased(user.ID, item.ID)
	if err != nil { return false, err }
	if alreadyPurchased { return false, nil }
	purchased, err := s.repo.PurchaseInspiration(item, user.ID)
	if errors.Is(err, repository.ErrInsufficientCredits) { return false, BadAuthRequest("积分不足，无法解锁这份灵感") }
	return purchased, err
}

func (s *Service) CreateInspiration(user *model.User, req CreateInspirationRequest) (*model.Inspiration, error) {
	if user == nil { return nil, Unauthorized("请先登录") }
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind != "image" && kind != "video" && kind != "short-drama" { return nil, BadAuthRequest("灵感类型仅支持图片、视频或短剧") }
	title := strings.TrimSpace(req.Title)
	prompt := strings.TrimSpace(req.Prompt)
	if title == "" || prompt == "" { return nil, BadAuthRequest("标题和提示词不能为空") }
	if req.PriceMicrocredits < 0 || req.PriceMicrocredits > maxInspirationPrice { return nil, BadAuthRequest("分享积分价格不合法") }
	previewURL := strings.TrimSpace(req.PreviewURL)
	previewResourceID := strings.TrimSpace(req.PreviewResourceID)
	if (kind == "image" || kind == "video") && previewURL == "" && previewResourceID == "" { return nil, BadAuthRequest("图片或视频分享必须包含预览内容") }
	if previewResourceID != "" {
		resource, resourceErr := s.repo.ResourceForUser(user.ID, previewResourceID)
		if resourceErr != nil { return nil, BadAuthRequest("分享的媒体资源不存在") }
		previewURL, resourceErr = s.directResourceURL(resource, time.Now().Add(365*24*time.Hour))
		if resourceErr != nil { return nil, resourceErr }
	}
	if req.Parameters == nil { req.Parameters = map[string]any{} }
	parametersJSON, err := json.Marshal(req.Parameters)
	if err != nil || len(parametersJSON) > 64<<10 { return nil, BadAuthRequest("创作参数不合法") }
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" { sourceType = "creation" }
	item := &model.Inspiration{ID: newID(), OwnerUserID: user.ID, Kind: kind, Title: truncateRunes(title, 160), Description: truncateRunes(strings.TrimSpace(req.Description), 500), PreviewURL: previewURL, PreviewResourceID: previewResourceID, Prompt: truncateRunes(prompt, 20_000), ParametersJSON: string(parametersJSON), SourceType: truncateRunes(sourceType, 32), SourceID: truncateRunes(strings.TrimSpace(req.SourceID), 80), PriceMicrocredits: req.PriceMicrocredits, Status: "published", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.repo.CreateInspiration(item); err != nil { return nil, err }
	return item, nil
}

func (s *Service) ReplicateInspiration(user *model.User, id string) (*InspirationReplica, error) {
	if user == nil { return nil, Unauthorized("请先登录") }
	item, err := s.repo.Inspiration(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) { return nil, &AuthError{Status: 404, Message: "灵感不存在或已下架"} }
	if err != nil { return nil, err }
	if item.PriceMicrocredits > 0 && item.OwnerUserID != user.ID {
		alreadyPurchased, purchaseErr := s.repo.InspirationPurchased(user.ID, item.ID)
		if purchaseErr != nil { return nil, purchaseErr }
		if !alreadyPurchased {
			return nil, &AuthError{Status: 402, Message: "请先确认支付积分并解锁这份灵感"}
		}
	} else if item.OwnerUserID != user.ID {
		if err := s.repo.IncrementInspirationCopyCount(item.ID); err != nil { return nil, err }
	}
	parameters := map[string]any{}
	if err := json.Unmarshal([]byte(item.ParametersJSON), &parameters); err != nil { return nil, errors.New("灵感参数损坏") }
	return &InspirationReplica{ID: item.ID, Kind: item.Kind, Prompt: item.Prompt, Parameters: parameters, SourceType: item.SourceType, SourceID: item.SourceID, Purchased: false}, nil
}
