package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// GateTriggerFn physically executes an open or close action on a gate.
// Errors are fire-and-forget: the implementation logs them internally.
type GateTriggerFn func(ctx context.Context, gate *model.Gate, action string)

// CreateGateParams holds the fields for creating a new gate.
type CreateGateParams struct {
	Name              string
	IntegrationType   model.GateIntegrationType
	IntegrationConfig map[string]any
	OpenConfig        *model.ActionConfig
	CloseConfig       *model.ActionConfig
	StatusConfig      *model.ActionConfig
	MetaConfig        []model.MetaField
	StatusRules       []model.StatusRule
}

// UpdateGateParams holds optional fields for updating a gate.
// For action configs, use model.OmittableNullable: Sent=false = unchanged, Null=true = clear to NULL.
type UpdateGateParams struct {
	Name         *string
	OpenConfig   model.OmittableNullable[model.ActionConfig]
	CloseConfig  model.OmittableNullable[model.ActionConfig]
	StatusConfig model.OmittableNullable[model.ActionConfig]
	MetaConfig   []model.MetaField  // nil = unchanged, [] = clear
	StatusRules  []model.StatusRule // nil = unchanged, [] = clear
}

type GateService struct {
	gates        repository.GateRepository
	policies     *PolicyService
	credPolicies repository.CredentialPolicyRepository
	schedules    *ScheduleService
	audit        repository.AuditRepository
	trigger      GateTriggerFn
	jwtSecret    []byte
	redis        *redis.Client
}

func NewGateService(
	gates repository.GateRepository,
	policies *PolicyService,
	credPolicies repository.CredentialPolicyRepository,
	schedules *ScheduleService,
	audit repository.AuditRepository,
	trigger GateTriggerFn,
	jwtSecret []byte,
	redis *redis.Client,
) *GateService {
	return &GateService{
		gates:        gates,
		policies:     policies,
		credPolicies: credPolicies,
		schedules:    schedules,
		audit:        audit,
		trigger:      trigger,
		jwtSecret:    jwtSecret,
		redis:        redis,
	}
}

// AuthenticateToken validates a JWT gate token (signature + DB rotation check)
// and returns the authenticated gate.
func (s *GateService) AuthenticateToken(ctx context.Context, token string) (*model.Gate, error) {
	gateID, _, err := ParseGateToken(token, s.jwtSecret)
	if err != nil {
		return nil, repository.ErrUnauthorized
	}
	gate, err := s.gates.GetByToken(ctx, gateID, token)
	if err != nil {
		return nil, err
	}
	return gate, nil
}

// ProcessStatus evaluates status rules and persists the gate status update.
func (s *GateService) ProcessStatus(ctx context.Context, gate *model.Gate, rawStatus string, meta map[string]any) error {
	return ProcessGateStatus(ctx, s.gates, s.redis, gate, rawStatus, meta)
}

func (s *GateService) Create(ctx context.Context, wsID uuid.UUID, params CreateGateParams) (*model.Gate, error) {
	intType := params.IntegrationType
	if intType == "" {
		intType = model.IntegrationTypeMQTT
	}
	gate, err := s.gates.Create(ctx, wsID, repository.CreateGateParams{
		Name:              params.Name,
		IntegrationType:   intType,
		IntegrationConfig: params.IntegrationConfig,
		OpenConfig:        params.OpenConfig,
		CloseConfig:       params.CloseConfig,
		StatusConfig:      params.StatusConfig,
		MetaConfig:        params.MetaConfig,
		StatusRules:       params.StatusRules,
	})
	if err != nil {
		return nil, fmt.Errorf("create gate: %w", err)
	}

	// Replace the DB-generated random token with a JWT containing gate identity.
	jwtToken, err := IssueGateToken(gate.ID, gate.WorkspaceID, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("issue gate token: %w", err)
	}
	if err := s.gates.SetToken(ctx, gate.ID, gate.WorkspaceID, jwtToken); err != nil {
		return nil, fmt.Errorf("set gate token: %w", err)
	}
	gate.GateToken = &jwtToken
	gate.Status = gate.EffectiveStatus()
	return gate, nil
}

func (s *GateService) List(ctx context.Context, wsID uuid.UUID, role model.WorkspaceRole, membershipID uuid.UUID) ([]model.Gate, error) {
	gates, err := s.gates.ListForWorkspace(ctx, wsID, role, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list gates: %w", err)
	}
	if gates == nil {
		gates = []model.Gate{}
	}
	for i := range gates {
		gates[i].Status = gates[i].EffectiveStatus()
	}
	return gates, nil
}

func (s *GateService) Get(ctx context.Context, gateID, wsID uuid.UUID, role model.WorkspaceRole, membershipID uuid.UUID) (*model.Gate, error) {
	gate, err := s.gates.GetByID(ctx, gateID, wsID)
	if err != nil {
		return nil, err
	}
	if role == model.RoleMember {
		ok, err := s.policies.HasAnyPermission(ctx, membershipID, gate.ID)
		if err != nil {
			return nil, fmt.Errorf("check permissions: %w", err)
		}
		if !ok {
			return nil, model.ErrUnauthorized
		}
		hasStatus, _ := s.policies.HasPermission(ctx, membershipID, gate.ID, "gate:read_status")
		if !hasStatus {
			gate.StatusMetadata = nil
		}
	}
	gate.Status = gate.EffectiveStatus()
	return gate, nil
}

func (s *GateService) Update(ctx context.Context, gateID, wsID uuid.UUID, params UpdateGateParams) (*model.Gate, error) {
	gate, err := s.gates.Update(ctx, gateID, wsID, repository.UpdateGateParams{
		Name:         params.Name,
		OpenConfig:   params.OpenConfig,
		CloseConfig:  params.CloseConfig,
		StatusConfig: params.StatusConfig,
		MetaConfig:   params.MetaConfig,
		StatusRules:  params.StatusRules,
	})
	if err != nil {
		return nil, err
	}
	gate.Status = gate.EffectiveStatus()
	return gate, nil
}

func (s *GateService) Delete(ctx context.Context, gateID, wsID uuid.UUID) error {
	return s.gates.Delete(ctx, gateID, wsID)
}

func (s *GateService) GetToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error) {
	return s.gates.GetToken(ctx, gateID, wsID)
}

// RotateToken issues a new JWT gate token and stores it, invalidating the previous one.
func (s *GateService) RotateToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error) {
	// Verify the gate exists and belongs to the workspace.
	gate, err := s.gates.GetByID(ctx, gateID, wsID)
	if err != nil {
		return "", err
	}
	jwtToken, err := IssueGateToken(gate.ID, gate.WorkspaceID, s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("issue gate token: %w", err)
	}
	if err := s.gates.SetToken(ctx, gateID, wsID, jwtToken); err != nil {
		return "", fmt.Errorf("set gate token: %w", err)
	}
	return jwtToken, nil
}

// Trigger executes an open or close action on a gate.
// For MEMBER role it checks the required permission and any attached schedule.
// If credentialID is set, also checks credential-level policies and schedule (AND logic with membership schedule).
func (s *GateService) Trigger(ctx context.Context, gateID, wsID uuid.UUID, role model.WorkspaceRole, membershipID, credentialID uuid.UUID, action string) error {
	permCode := "gate:trigger_open"
	if action == "close" {
		permCode = "gate:trigger_close"
	}

	// 1. Check credential-level schedule (user-defined time restriction on the token).
	if credentialID != uuid.Nil {
		if scheduleID, err := s.credPolicies.GetScheduleID(ctx, credentialID); err == nil {
			if schedule, err := s.schedules.GetPublic(ctx, scheduleID); err == nil {
				if err := s.schedules.Check(schedule, time.Now()); err != nil {
					slog.Info("gate trigger: credential schedule rejected", "credential_id", credentialID, "gate_id", gateID)
					return ErrScheduleDenied
				}
			}
		}
	}

	// 2. Permission check: credential policies take precedence if set, else fall back to membership policies.
	needsMembershipCheck := role == model.RoleMember
	if credentialID != uuid.Nil {
		hasCredPolicies, _ := s.credPolicies.HasAny(ctx, credentialID)
		if hasCredPolicies {
			ok, _ := s.credPolicies.HasPermission(ctx, credentialID, gateID, permCode)
			if !ok {
				return model.ErrUnauthorized
			}
			needsMembershipCheck = false
		}
	}
	if needsMembershipCheck {
		ok, err := s.policies.HasPermission(ctx, membershipID, gateID, permCode)
		if err != nil {
			return fmt.Errorf("check permission: %w", err)
		}
		if !ok {
			return model.ErrUnauthorized
		}
	}

	// 3. Membership-level schedule: admin-defined ceiling — always applies for members and token users.
	if role == model.RoleMember || credentialID != uuid.Nil {
		if scheduleID, err := s.policies.GetMemberGateScheduleID(ctx, membershipID, gateID); err == nil {
			if schedule, err := s.schedules.GetPublic(ctx, scheduleID); err == nil {
				if err := s.schedules.Check(schedule, time.Now()); err != nil {
					slog.Info("gate trigger: membership schedule rejected", "membership_id", membershipID, "gate_id", gateID)
					return ErrScheduleDenied
				}
			}
		}
	}

	gate, err := s.gates.GetByID(ctx, gateID, wsID)
	if err != nil {
		return err
	}

	s.trigger(ctx, gate, action)

	auditAction := "gate:trigger_open"
	if action == "close" {
		auditAction = "gate:trigger_close"
	}
	_ = s.audit.Insert(ctx, repository.AuditEntry{
		WorkspaceID: wsID,
		GateID:      &gateID,
		Action:      auditAction,
	})
	return nil
}
