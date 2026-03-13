package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// GateTriggerFn physically executes an open or close action on a gate.
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
	CustomStatuses    []string
	TTLSeconds        *int
	StatusTransitions []model.StatusTransition
}

// UpdateGateParams holds optional fields for updating a gate.
type UpdateGateParams struct {
	Name              *string
	OpenConfig        model.OmittableNullable[model.ActionConfig]
	CloseConfig       model.OmittableNullable[model.ActionConfig]
	StatusConfig      model.OmittableNullable[model.ActionConfig]
	MetaConfig        []model.MetaField
	StatusRules       []model.StatusRule
	CustomStatuses    []string
	TTLSeconds        model.OmittableNullable[int]
	StatusTransitions []model.StatusTransition
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
	brokerAuth   BrokerAuthManager
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
	brokerAuth BrokerAuthManager,
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
		brokerAuth:   brokerAuth,
	}
}

// AuthenticateToken validates a JWT gate token and returns the authenticated gate.
func (s *GateService) AuthenticateToken(ctx context.Context, token string) (*model.Gate, error) {
	gateID, err := ParseGateToken(token, s.jwtSecret)
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

func (s *GateService) Create(ctx context.Context, params CreateGateParams) (*model.Gate, error) {
	intType := params.IntegrationType
	if intType == "" {
		intType = model.IntegrationTypeMQTT
	}
	gate, err := s.gates.Create(ctx, repository.CreateGateParams{
		Name:              params.Name,
		IntegrationType:   intType,
		IntegrationConfig: params.IntegrationConfig,
		OpenConfig:        params.OpenConfig,
		CloseConfig:       params.CloseConfig,
		StatusConfig:      params.StatusConfig,
		MetaConfig:        params.MetaConfig,
		StatusRules:       params.StatusRules,
		CustomStatuses:    params.CustomStatuses,
		TTLSeconds:        params.TTLSeconds,
		StatusTransitions: params.StatusTransitions,
	})
	if err != nil {
		return nil, fmt.Errorf("create gate: %w", err)
	}

	jwtToken, err := IssueGateToken(gate.ID, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("issue gate token: %w", err)
	}
	if err := s.gates.SetToken(ctx, gate.ID, jwtToken); err != nil {
		return nil, fmt.Errorf("set gate token: %w", err)
	}

	if err := s.brokerAuth.SyncCredentials(ctx, gate.ID, jwtToken); err != nil {
		slog.Warn("gate: failed to sync broker credentials (gate still created)", "gate_id", gate.ID, "error", err)
	}

	gate.GateToken = &jwtToken
	gate.Status = gate.EffectiveStatus()
	return gate, nil
}

func (s *GateService) List(ctx context.Context, role model.Role, memberID uuid.UUID, p model.PaginationParams) ([]model.Gate, int, error) {
	gates, total, err := s.gates.List(ctx, role, memberID, p)
	if err != nil {
		return nil, 0, fmt.Errorf("list gates: %w", err)
	}
	if gates == nil {
		gates = []model.Gate{}
	}
	for i := range gates {
		gates[i].Status = gates[i].EffectiveStatus()
	}
	return gates, total, nil
}

func (s *GateService) Get(ctx context.Context, gateID uuid.UUID, role model.Role, memberID uuid.UUID) (*model.Gate, error) {
	gate, err := s.gates.GetByID(ctx, gateID)
	if err != nil {
		return nil, err
	}
	if role == model.RoleMember {
		ok, err := s.policies.HasAnyPermission(ctx, memberID, gate.ID)
		if err != nil {
			return nil, fmt.Errorf("check permissions: %w", err)
		}
		if !ok {
			return nil, model.ErrUnauthorized
		}
		hasStatus, _ := s.policies.HasPermission(ctx, memberID, gate.ID, "gate:read_status")
		if !hasStatus {
			gate.StatusMetadata = nil
		}
	}
	gate.Status = gate.EffectiveStatus()
	return gate, nil
}

func (s *GateService) Update(ctx context.Context, gateID uuid.UUID, params UpdateGateParams) (*model.Gate, error) {
	gate, err := s.gates.Update(ctx, gateID, repository.UpdateGateParams{
		Name:              params.Name,
		OpenConfig:        params.OpenConfig,
		CloseConfig:       params.CloseConfig,
		StatusConfig:      params.StatusConfig,
		MetaConfig:        params.MetaConfig,
		StatusRules:       params.StatusRules,
		CustomStatuses:    params.CustomStatuses,
		TTLSeconds:        params.TTLSeconds,
		StatusTransitions: params.StatusTransitions,
	})
	if err != nil {
		return nil, err
	}
	gate.Status = gate.EffectiveStatus()
	return gate, nil
}

func (s *GateService) Delete(ctx context.Context, gateID uuid.UUID) error {
	if err := s.gates.Delete(ctx, gateID); err != nil {
		return err
	}
	if err := s.brokerAuth.RemoveCredentials(ctx, gateID); err != nil {
		slog.Warn("gate: failed to remove broker credentials (gate already deleted)", "gate_id", gateID, "error", err)
	}
	return nil
}

func (s *GateService) GetToken(ctx context.Context, gateID uuid.UUID) (string, error) {
	return s.gates.GetToken(ctx, gateID)
}

// RotateToken issues a new JWT gate token and stores it.
func (s *GateService) RotateToken(ctx context.Context, gateID uuid.UUID) (string, error) {
	gate, err := s.gates.GetByID(ctx, gateID)
	if err != nil {
		return "", err
	}
	jwtToken, err := IssueGateToken(gate.ID, s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("issue gate token: %w", err)
	}
	if err := s.gates.SetToken(ctx, gateID, jwtToken); err != nil {
		return "", fmt.Errorf("set gate token: %w", err)
	}

	if err := s.brokerAuth.SyncCredentials(ctx, gate.ID, jwtToken); err != nil {
		slog.Warn("gate: failed to sync broker credentials after rotation", "gate_id", gateID, "error", err)
	}

	return jwtToken, nil
}

// Trigger executes an open or close action on a gate.
func (s *GateService) Trigger(ctx context.Context, gateID uuid.UUID, role model.Role, memberID, credentialID uuid.UUID, action string) error {
	permCode := "gate:trigger_open"
	if action == "close" {
		permCode = "gate:trigger_close"
	}

	if credentialID != uuid.Nil {
		scheduleID, err := s.credPolicies.GetScheduleID(ctx, credentialID)
		if err == nil {
			schedule, err := s.schedules.GetPublic(ctx, scheduleID)
			if err != nil {
				slog.Error("gate trigger: failed to load credential schedule", "credential_id", credentialID, "schedule_id", scheduleID, "error", err)
				return fmt.Errorf("load credential schedule: %w", err)
			}
			if err := s.schedules.Check(schedule, time.Now()); err != nil {
				slog.Info("gate trigger: credential schedule rejected", "credential_id", credentialID, "gate_id", gateID)
				return ErrScheduleDenied
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			slog.Error("gate trigger: failed to lookup credential schedule", "credential_id", credentialID, "error", err)
			return fmt.Errorf("lookup credential schedule: %w", err)
		}
	}

	needsMemberCheck := role == model.RoleMember
	if credentialID != uuid.Nil {
		hasCredPolicies, err := s.credPolicies.HasAny(ctx, credentialID)
		if err != nil {
			return fmt.Errorf("check credential policies: %w", err)
		}
		if hasCredPolicies {
			ok, err := s.credPolicies.HasPermission(ctx, credentialID, gateID, permCode)
			if err != nil {
				return fmt.Errorf("check credential permission: %w", err)
			}
			if !ok {
				return model.ErrUnauthorized
			}
			needsMemberCheck = false
		}
	}
	if needsMemberCheck {
		ok, err := s.policies.HasPermission(ctx, memberID, gateID, permCode)
		if err != nil {
			return fmt.Errorf("check permission: %w", err)
		}
		if !ok {
			return model.ErrUnauthorized
		}
	}

	if role == model.RoleMember || credentialID != uuid.Nil {
		scheduleID, err := s.policies.GetMemberGateScheduleID(ctx, memberID, gateID)
		if err == nil {
			schedule, err := s.schedules.GetPublic(ctx, scheduleID)
			if err != nil {
				slog.Error("gate trigger: failed to load member schedule", "member_id", memberID, "schedule_id", scheduleID, "error", err)
				return fmt.Errorf("load member schedule: %w", err)
			}
			if err := s.schedules.Check(schedule, time.Now()); err != nil {
				slog.Info("gate trigger: member schedule rejected", "member_id", memberID, "gate_id", gateID)
				return ErrScheduleDenied
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			slog.Error("gate trigger: failed to lookup member schedule", "member_id", memberID, "gate_id", gateID, "error", err)
			return fmt.Errorf("lookup member schedule: %w", err)
		}
	}

	gate, err := s.gates.GetByID(ctx, gateID)
	if err != nil {
		return err
	}

	s.trigger(ctx, gate, action)

	auditAction := "gate:trigger_open"
	if action == "close" {
		auditAction = "gate:trigger_close"
	}
	_ = s.audit.Insert(ctx, repository.AuditEntry{
		GateID:   &gateID,
		MemberID: &memberID,
		Action:   auditAction,
	})
	return nil
}
