package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PolicyRepository struct {
	pool *pgxpool.Pool
}

func NewPolicyRepository(pool *pgxpool.Pool) *PolicyRepository {
	return &PolicyRepository{pool: pool}
}

func (r *PolicyRepository) List(ctx context.Context, gateID uuid.UUID) ([]model.MembershipPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT membership_id, gate_id, permission_code FROM membership_policies
		 WHERE gate_id = $1
		 ORDER BY membership_id, permission_code`,
		gateID,
	)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	var result []model.MembershipPolicy
	for rows.Next() {
		var p model.MembershipPolicy
		if err := rows.Scan(&p.MembershipID, &p.GateID, &p.PermissionCode); err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *PolicyRepository) ListForMembership(ctx context.Context, membershipID uuid.UUID) ([]model.MembershipPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT membership_id, gate_id, permission_code FROM membership_policies
		 WHERE membership_id = $1
		 ORDER BY gate_id, permission_code`,
		membershipID,
	)
	if err != nil {
		return nil, fmt.Errorf("list membership policies: %w", err)
	}
	defer rows.Close()

	var result []model.MembershipPolicy
	for rows.Next() {
		var p model.MembershipPolicy
		if err := rows.Scan(&p.MembershipID, &p.GateID, &p.PermissionCode); err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// Grant adds a permission for a membership on a gate. Idempotent (ON CONFLICT DO NOTHING).
func (r *PolicyRepository) Grant(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO membership_policies (membership_id, gate_id, permission_code)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		membershipID, gateID, permCode,
	)
	if err != nil {
		return fmt.Errorf("grant policy: %w", err)
	}
	return nil
}

// HasPermission returns true if the membership has the given permission on the gate.
func (r *PolicyRepository) HasPermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM membership_policies
		  WHERE membership_id = $1 AND gate_id = $2 AND permission_code = $3)`,
		membershipID, gateID, permCode,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return exists, nil
}

// HasAnyPermission returns true if the membership has at least one policy on the gate.
func (r *PolicyRepository) HasAnyPermission(ctx context.Context, membershipID, gateID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM membership_policies
		  WHERE membership_id = $1 AND gate_id = $2)`,
		membershipID, gateID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check any permission: %w", err)
	}
	return exists, nil
}

// Revoke removes all permissions for a membership on a gate.
func (r *PolicyRepository) Revoke(ctx context.Context, membershipID, gateID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM membership_policies WHERE membership_id = $1 AND gate_id = $2`,
		membershipID, gateID,
	)
	if err != nil {
		return fmt.Errorf("revoke policy: %w", err)
	}
	return nil
}

// RevokePermission removes a specific permission for a membership on a gate.
func (r *PolicyRepository) RevokePermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM membership_policies
		 WHERE membership_id = $1 AND gate_id = $2 AND permission_code = $3`,
		membershipID, gateID, permCode,
	)
	if err != nil {
		return fmt.Errorf("revoke permission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetMemberGateSchedule attaches (or replaces) a time-restriction schedule to a member-gate pair.
func (r *PolicyRepository) SetMemberGateSchedule(ctx context.Context, membershipID, gateID, scheduleID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO membership_gate_schedules (membership_id, gate_id, schedule_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (membership_id, gate_id) DO UPDATE SET schedule_id = EXCLUDED.schedule_id`,
		membershipID, gateID, scheduleID,
	)
	if err != nil {
		return fmt.Errorf("set member gate schedule: %w", err)
	}
	return nil
}

// RemoveMemberGateSchedule detaches any schedule from a member-gate pair (makes access unrestricted).
func (r *PolicyRepository) RemoveMemberGateSchedule(ctx context.Context, membershipID, gateID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM membership_gate_schedules WHERE membership_id = $1 AND gate_id = $2`,
		membershipID, gateID,
	)
	if err != nil {
		return fmt.Errorf("remove member gate schedule: %w", err)
	}
	return nil
}

// GetMemberGateScheduleID returns the schedule_id attached to a member-gate pair, or ErrNotFound if none.
func (r *PolicyRepository) GetMemberGateScheduleID(ctx context.Context, membershipID, gateID uuid.UUID) (uuid.UUID, error) {
	var scheduleID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT schedule_id FROM membership_gate_schedules WHERE membership_id = $1 AND gate_id = $2`,
		membershipID, gateID,
	).Scan(&scheduleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get member gate schedule: %w", err)
	}
	return scheduleID, nil
}
