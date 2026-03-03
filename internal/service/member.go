package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUsernameTaken = errors.New("username already taken in this workspace")
	ErrAlreadyMember = errors.New("user already has a membership in this workspace")
)

type MembershipService struct {
	memberships *repository.WorkspaceMembershipRepository
	memberCreds *repository.MembershipCredentialRepository
	workspaces  *repository.WorkspaceRepository
}

func NewMembershipService(
	memberships *repository.WorkspaceMembershipRepository,
	memberCreds *repository.MembershipCredentialRepository,
	workspaces *repository.WorkspaceRepository,
) *MembershipService {
	return &MembershipService{
		memberships: memberships,
		memberCreds: memberCreds,
		workspaces:  workspaces,
	}
}

// CreateLocal creates a managed (local) membership with a password credential.
func (s *MembershipService) CreateLocal(ctx context.Context, workspaceID uuid.UUID, localUsername string, displayName *string, password string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	membership, err := s.memberships.CreateLocal(ctx, workspaceID, localUsername, displayName, role, invitedBy)
	if errors.Is(err, repository.ErrAlreadyExists) {
		return nil, ErrUsernameTaken
	}
	if err != nil {
		return nil, fmt.Errorf("create membership: %w", err)
	}

	_, err = s.memberCreds.Create(ctx, membership.ID, model.CredPassword, string(hashed), nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create password credential: %w", err)
	}

	return membership, nil
}

func (s *MembershipService) GetByID(ctx context.Context, membershipID, workspaceID uuid.UUID) (*model.WorkspaceMembership, error) {
	return s.memberships.GetByID(ctx, membershipID, workspaceID)
}

func (s *MembershipService) List(ctx context.Context, workspaceID uuid.UUID) ([]*model.WorkspaceMembership, error) {
	return s.memberships.List(ctx, workspaceID)
}

func (s *MembershipService) Update(ctx context.Context, membershipID, workspaceID uuid.UUID, displayName *string, role model.WorkspaceRole, authConfig map[string]any) (*model.WorkspaceMembership, error) {
	return s.memberships.Update(ctx, membershipID, workspaceID, displayName, role, authConfig)
}

func (s *MembershipService) Delete(ctx context.Context, membershipID, workspaceID uuid.UUID) error {
	return s.memberships.Delete(ctx, membershipID, workspaceID)
}

func (s *MembershipService) SetPassword(ctx context.Context, membershipID, workspaceID uuid.UUID, password string) error {
	// Verify membership exists in this workspace.
	if _, err := s.memberships.GetByID(ctx, membershipID, workspaceID); err != nil {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Delete existing password credential (if any) then create a new one.
	existing, err := s.memberCreds.GetByMembershipAndType(ctx, membershipID, model.CredPassword)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("get existing credential: %w", err)
	}
	if existing != nil {
		if err := s.memberCreds.Delete(ctx, existing.ID, membershipID); err != nil {
			return fmt.Errorf("delete existing credential: %w", err)
		}
	}

	_, err = s.memberCreds.Create(ctx, membershipID, model.CredPassword, string(hashed), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("create password credential: %w", err)
	}
	return nil
}

// InviteUser creates a membership for an existing platform user (no password).
func (s *MembershipService) InviteUser(ctx context.Context, workspaceID, userID uuid.UUID, displayName *string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error) {
	membership, err := s.memberships.CreateForUser(ctx, workspaceID, userID, displayName, role, invitedBy)
	if errors.Is(err, repository.ErrAlreadyExists) {
		return nil, ErrAlreadyMember
	}
	if err != nil {
		return nil, fmt.Errorf("invite user: %w", err)
	}
	return membership, nil
}

// GetEffectiveAuthConfig merges the workspace-level member_auth_config with the membership-level
// auth_config override. Membership values take precedence; null values in auth_config inherit from workspace.
func GetEffectiveAuthConfig(workspace *model.Workspace, membership *model.WorkspaceMembership) map[string]any {
	result := make(map[string]any, len(workspace.MemberAuthConfig))
	for k, v := range workspace.MemberAuthConfig {
		result[k] = v
	}
	for k, v := range membership.AuthConfig {
		if v != nil {
			result[k] = v
		}
	}
	return result
}
