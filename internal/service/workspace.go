package service

import (
	"context"
	"fmt"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
)

type WorkspaceService struct {
	workspaces repository.WorkspaceRepository
}

func NewWorkspaceService(workspaces repository.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{workspaces: workspaces}
}

func (s *WorkspaceService) Create(ctx context.Context, name string, ownerID uuid.UUID) (*model.WorkspaceWithRole, error) {
	ws, err := s.workspaces.Create(ctx, name, ownerID)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &model.WorkspaceWithRole{Workspace: *ws, Role: model.RoleOwner}, nil
}

func (s *WorkspaceService) List(ctx context.Context, userID uuid.UUID) ([]model.WorkspaceWithRole, error) {
	list, err := s.workspaces.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	if list == nil {
		list = []model.WorkspaceWithRole{}
	}
	return list, nil
}

func (s *WorkspaceService) Get(ctx context.Context, wsID, userID uuid.UUID) (*model.WorkspaceWithRole, error) {
	ws, err := s.workspaces.GetByID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	role, err := s.workspaces.GetMemberRole(ctx, ws.ID, userID)
	if err != nil {
		return nil, err
	}
	return &model.WorkspaceWithRole{Workspace: *ws, Role: role}, nil
}

func (s *WorkspaceService) Rename(ctx context.Context, wsID uuid.UUID, name string) (*model.Workspace, error) {
	ws, err := s.workspaces.Rename(ctx, wsID, name)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (s *WorkspaceService) Delete(ctx context.Context, wsID uuid.UUID) error {
	return s.workspaces.Delete(ctx, wsID)
}

func (s *WorkspaceService) GetMemberAuthConfig(ctx context.Context, wsID uuid.UUID) (map[string]any, error) {
	ws, err := s.workspaces.GetByID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	cfg := ws.MemberAuthConfig
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}

func (s *WorkspaceService) UpdateMemberAuthConfig(ctx context.Context, wsID uuid.UUID, config map[string]any) (map[string]any, error) {
	ws, err := s.workspaces.UpdateMemberAuthConfig(ctx, wsID, config)
	if err != nil {
		return nil, err
	}
	cfg := ws.MemberAuthConfig
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}
