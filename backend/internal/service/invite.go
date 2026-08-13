package service

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type InviteSummary struct {
	Code                      string `json:"code"`
	InvitedCount              int64  `json:"invitedCount"`
	InviterRewardMicrocredits int64  `json:"inviterRewardMicrocredits"`
	InviteeRewardMicrocredits int64  `json:"inviteeRewardMicrocredits"`
	Enabled                   bool   `json:"enabled"`
}

func (s *Service) InviteSummary(user *model.User) (*InviteSummary, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	code, err := s.ensureUserInviteCode(user.ID)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.InviteReferralCount(user.ID)
	if err != nil {
		return nil, err
	}
	policy, err := s.creditPolicy()
	if err != nil {
		return nil, err
	}
	return &InviteSummary{Code: code.Code, InvitedCount: count, InviterRewardMicrocredits: policy.InviterRewardMicrocredits, InviteeRewardMicrocredits: policy.InviteeRewardMicrocredits, Enabled: policy.InviteEnabled}, nil
}

func (s *Service) applyInviteCode(inviteeID, rawCode string) error {
	codeValue := strings.ToUpper(strings.TrimSpace(rawCode))
	if codeValue == "" {
		return nil
	}
	code, policy, err := s.validateInviteCode(inviteeID, codeValue)
	if err != nil {
		return err
	}
	_, err = s.repo.ApplyInviteReward(code.UserID, inviteeID, code.Code, policy.InviterRewardMicrocredits, policy.InviteeRewardMicrocredits)
	return err
}

func (s *Service) validateInviteCode(inviteeID, rawCode string) (*model.UserInviteCode, CreditPolicy, error) {
	policy, err := s.creditPolicy()
	if err != nil {
		return nil, CreditPolicy{}, err
	}
	if !policy.InviteEnabled {
		return nil, CreditPolicy{}, BadAuthRequest("邀请奖励当前未开放")
	}
	code, err := s.repo.InviteCode(strings.ToUpper(strings.TrimSpace(rawCode)))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, CreditPolicy{}, BadAuthRequest("邀请码不存在")
	}
	if err != nil {
		return nil, CreditPolicy{}, err
	}
	if inviteeID != "" && code.UserID == inviteeID {
		return nil, CreditPolicy{}, BadAuthRequest("不能使用自己的邀请码")
	}
	return code, policy, nil
}

func (s *Service) ensureUserInviteCode(userID string) (*model.UserInviteCode, error) {
	item, err := s.repo.InviteCodeForUser(userID)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		code := strings.ToUpper(randomToken()[:8])
		item = &model.UserInviteCode{ID: newID(), UserID: userID, Code: code, CreatedAt: time.Now()}
		if err := s.repo.CreateInviteCode(item); err == nil {
			return item, nil
		}
	}
	return nil, errors.New("生成邀请码失败，请稍后重试")
}
