package service

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
)

type PolicyService struct {
	policies  repository.PolicyRepository
	schedules repository.AccessScheduleRepository
}

func NewPolicyService(policies repository.PolicyRepository, schedules repository.AccessScheduleRepository) *PolicyService {
	return &PolicyService{policies: policies, schedules: schedules}
}

func (s *PolicyService) ListForGate(ctx context.Context, gateID uuid.UUID, p model.PaginationParams) ([]model.MemberPolicy, int, error) {
	policies, total, err := s.policies.ListForGate(ctx, gateID, p)
	if err != nil {
		return nil, 0, err
	}
	if policies == nil {
		policies = []model.MemberPolicy{}
	}
	return policies, total, nil
}

func (s *PolicyService) ListForMember(ctx context.Context, memberID uuid.UUID, p model.PaginationParams) ([]model.MemberPolicy, int, error) {
	policies, total, err := s.policies.ListForMember(ctx, memberID, p)
	if err != nil {
		return nil, 0, err
	}
	if policies == nil {
		policies = []model.MemberPolicy{}
	}
	return policies, total, nil
}

func (s *PolicyService) Grant(ctx context.Context, memberID, gateID uuid.UUID, permCode string) error {
	return s.policies.Grant(ctx, memberID, gateID, permCode)
}

func (s *PolicyService) Revoke(ctx context.Context, memberID, gateID uuid.UUID) error {
	return s.policies.Revoke(ctx, memberID, gateID)
}

func (s *PolicyService) RevokePermission(ctx context.Context, memberID, gateID uuid.UUID, permCode string) error {
	return s.policies.RevokePermission(ctx, memberID, gateID, permCode)
}

func (s *PolicyService) HasPermission(ctx context.Context, memberID, gateID uuid.UUID, permCode string) (bool, error) {
	return s.policies.HasPermission(ctx, memberID, gateID, permCode)
}

func (s *PolicyService) HasAnyPermission(ctx context.Context, memberID, gateID uuid.UUID) (bool, error) {
	return s.policies.HasAnyPermission(ctx, memberID, gateID)
}

func (s *PolicyService) SetMemberGateSchedule(ctx context.Context, memberID, gateID, scheduleID uuid.UUID) error {
	return s.policies.SetSchedule(ctx, memberID, gateID, scheduleID)
}

func (s *PolicyService) RemoveMemberGateSchedule(ctx context.Context, memberID, gateID uuid.UUID) error {
	return s.policies.RemoveSchedule(ctx, memberID, gateID)
}

func (s *PolicyService) GetMemberGateScheduleID(ctx context.Context, memberID, gateID uuid.UUID) (uuid.UUID, error) {
	return s.policies.GetScheduleID(ctx, memberID, gateID)
}

func (s *PolicyService) GetMemberGateSchedule(ctx context.Context, memberID, gateID uuid.UUID) (*model.AccessSchedule, error) {
	scheduleID, err := s.policies.GetScheduleID(ctx, memberID, gateID)
	if err != nil {
		return nil, err
	}
	return s.schedules.GetByIDPublic(ctx, scheduleID)
}
