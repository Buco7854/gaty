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

func (s *PolicyService) List(ctx context.Context, gateID uuid.UUID) ([]model.MembershipPolicy, error) {
	policies, err := s.policies.List(ctx, gateID)
	if err != nil {
		return nil, err
	}
	if policies == nil {
		policies = []model.MembershipPolicy{}
	}
	return policies, nil
}

func (s *PolicyService) ListForMembership(ctx context.Context, membershipID uuid.UUID) ([]model.MembershipPolicy, error) {
	policies, err := s.policies.ListForMembership(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	if policies == nil {
		policies = []model.MembershipPolicy{}
	}
	return policies, nil
}

func (s *PolicyService) Grant(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error {
	return s.policies.Grant(ctx, membershipID, gateID, permCode)
}

func (s *PolicyService) Revoke(ctx context.Context, membershipID, gateID uuid.UUID) error {
	return s.policies.Revoke(ctx, membershipID, gateID)
}

func (s *PolicyService) RevokePermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error {
	return s.policies.RevokePermission(ctx, membershipID, gateID, permCode)
}

func (s *PolicyService) HasPermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) (bool, error) {
	return s.policies.HasPermission(ctx, membershipID, gateID, permCode)
}

func (s *PolicyService) HasAnyPermission(ctx context.Context, membershipID, gateID uuid.UUID) (bool, error) {
	return s.policies.HasAnyPermission(ctx, membershipID, gateID)
}

func (s *PolicyService) SetMemberGateSchedule(ctx context.Context, membershipID, gateID, scheduleID uuid.UUID) error {
	return s.policies.SetMemberGateSchedule(ctx, membershipID, gateID, scheduleID)
}

func (s *PolicyService) RemoveMemberGateSchedule(ctx context.Context, membershipID, gateID uuid.UUID) error {
	return s.policies.RemoveMemberGateSchedule(ctx, membershipID, gateID)
}

// GetMemberGateScheduleID returns the schedule ID attached to a member-gate pair.
func (s *PolicyService) GetMemberGateScheduleID(ctx context.Context, membershipID, gateID uuid.UUID) (uuid.UUID, error) {
	return s.policies.GetMemberGateScheduleID(ctx, membershipID, gateID)
}

// GetMemberGateSchedule returns the full schedule attached to a member-gate pair, or nil if none.
func (s *PolicyService) GetMemberGateSchedule(ctx context.Context, membershipID, gateID uuid.UUID) (*model.AccessSchedule, error) {
	scheduleID, err := s.policies.GetMemberGateScheduleID(ctx, membershipID, gateID)
	if err != nil {
		return nil, err
	}
	return s.schedules.GetByIDPublic(ctx, scheduleID)
}
