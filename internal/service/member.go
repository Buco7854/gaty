package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUsernameTaken = errors.New("username already taken in this workspace")
	ErrAlreadyMember = errors.New("user already has a membership in this workspace")
)

// UpdateMemberParams holds the optional fields for a membership PATCH.
// AuthConfig uses model.OmittableNullable: Sent=false = unchanged, Null=true = reset to NULL (inherit from workspace).
type UpdateMemberParams struct {
	DisplayName   *string
	LocalUsername *string
	Role          *model.WorkspaceRole
	AuthConfig    model.OmittableNullable[map[string]any]
}

type MembershipService struct {
	memberships repository.WorkspaceMembershipRepository
	memberCreds repository.MembershipCredentialRepository
	workspaces  repository.WorkspaceRepository
	gates       repository.GateRepository
	policies    repository.PolicyRepository
}

func NewMembershipService(
	memberships repository.WorkspaceMembershipRepository,
	memberCreds repository.MembershipCredentialRepository,
	workspaces repository.WorkspaceRepository,
	gates repository.GateRepository,
	policies repository.PolicyRepository,
) *MembershipService {
	return &MembershipService{
		memberships: memberships,
		memberCreds: memberCreds,
		workspaces:  workspaces,
		gates:       gates,
		policies:    policies,
	}
}

// CreateLocal creates a managed (local) membership with a password credential.
func (s *MembershipService) CreateLocal(ctx context.Context, workspaceID uuid.UUID, localUsername string, displayName *string, password string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	membership, err := s.memberships.CreateLocal(ctx, workspaceID, localUsername, displayName, role, invitedBy)
	if errors.Is(err, model.ErrAlreadyExists) {
		return nil, ErrUsernameTaken
	}
	if err != nil {
		return nil, fmt.Errorf("create membership: %w", err)
	}

	_, err = s.memberCreds.Create(ctx, membership.ID, model.CredPassword, string(hashed), nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create password credential: %w", err)
	}

	if err := s.applyDefaultPermissions(ctx, workspaceID, membership.ID); err != nil {
		fmt.Printf("warn: apply default permissions for membership %s: %v\n", membership.ID, err)
	}

	return membership, nil
}

func (s *MembershipService) GetByID(ctx context.Context, membershipID, workspaceID uuid.UUID) (*model.WorkspaceMembership, error) {
	return s.memberships.GetByID(ctx, membershipID, workspaceID)
}

func (s *MembershipService) List(ctx context.Context, workspaceID uuid.UUID) ([]*model.WorkspaceMembership, error) {
	members, err := s.memberships.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if members == nil {
		members = []*model.WorkspaceMembership{}
	}
	return members, nil
}

func (s *MembershipService) Update(ctx context.Context, membershipID, workspaceID uuid.UUID, params UpdateMemberParams) (*model.WorkspaceMembership, error) {
	return s.memberships.Update(ctx, membershipID, workspaceID,
		params.DisplayName,
		params.LocalUsername,
		params.Role,
		params.AuthConfig,
	)
}

func (s *MembershipService) Delete(ctx context.Context, membershipID, workspaceID uuid.UUID) error {
	return s.memberships.Delete(ctx, membershipID, workspaceID)
}

func (s *MembershipService) SetPassword(ctx context.Context, membershipID, workspaceID uuid.UUID, password string) error {
	if _, err := s.memberships.GetByID(ctx, membershipID, workspaceID); err != nil {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	existing, err := s.memberCreds.GetByMembershipAndType(ctx, membershipID, model.CredPassword)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
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
	if errors.Is(err, model.ErrAlreadyExists) {
		return nil, ErrAlreadyMember
	}
	if err != nil {
		return nil, fmt.Errorf("invite user: %w", err)
	}
	if err := s.applyDefaultPermissions(ctx, workspaceID, membership.ID); err != nil {
		// Non-fatal: membership is created, permissions can be set manually.
		fmt.Printf("warn: apply default permissions for membership %s: %v\n", membership.ID, err)
	}
	return membership, nil
}

// applyDefaultPermissions grants workspace-default per-gate permissions to a newly created membership.
// member_auth_config["default_gate_permissions"] is expected to be:
//
//	[{"gate_id": "<uuid>", "permissions": ["gate:read_status", ...]}, ...]
func (s *MembershipService) applyDefaultPermissions(ctx context.Context, workspaceID, membershipID uuid.UUID) error {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}
	raw, ok := ws.MemberAuthConfig["default_gate_permissions"]
	if !ok || raw == nil {
		return nil
	}
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		gateIDStr, ok := m["gate_id"].(string)
		if !ok {
			continue
		}
		gateID, err := uuid.Parse(gateIDStr)
		if err != nil {
			continue
		}
		permsAny, ok := m["permissions"].([]any)
		if !ok {
			continue
		}
		for _, p := range permsAny {
			perm, ok := p.(string)
			if !ok {
				continue
			}
			if err := s.policies.Grant(ctx, membershipID, gateID, perm); err != nil {
				return fmt.Errorf("grant %s on gate %s: %w", perm, gateID, err)
			}
		}
	}
	return nil
}

// GetEffectiveAuthConfig merges workspace-level member_auth_config with membership-level override.
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
