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

type MemberService struct {
	members     repository.MemberRepository
	credentials repository.CredentialRepository
}

func NewMemberService(
	members repository.MemberRepository,
	credentials repository.CredentialRepository,
) *MemberService {
	return &MemberService{
		members:     members,
		credentials: credentials,
	}
}

// Create creates a new member with a password credential.
func (s *MemberService) Create(ctx context.Context, username string, displayName *string, password string, role model.Role) (*model.Member, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	member, err := s.members.Create(ctx, username, displayName, role)
	if errors.Is(err, model.ErrAlreadyExists) {
		return nil, ErrUsernameTaken
	}
	if err != nil {
		return nil, fmt.Errorf("create member: %w", err)
	}

	_, err = s.credentials.Create(ctx, member.ID, model.CredPassword, string(hashed), nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create password credential: %w", err)
	}

	return member, nil
}

func (s *MemberService) GetByID(ctx context.Context, memberID uuid.UUID) (*model.Member, error) {
	return s.members.GetByID(ctx, memberID)
}

func (s *MemberService) List(ctx context.Context, p model.PaginationParams) ([]*model.Member, int, error) {
	members, total, err := s.members.List(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	if members == nil {
		members = []*model.Member{}
	}
	return members, total, nil
}

func (s *MemberService) Update(ctx context.Context, memberID uuid.UUID, displayName *string, username *string, role *model.Role) (*model.Member, error) {
	return s.members.Update(ctx, memberID, displayName, username, role)
}

func (s *MemberService) Delete(ctx context.Context, memberID uuid.UUID) error {
	return s.members.Delete(ctx, memberID)
}

func (s *MemberService) SetPassword(ctx context.Context, memberID uuid.UUID, password string) error {
	if _, err := s.members.GetByID(ctx, memberID); err != nil {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	existing, err := s.credentials.GetByMemberAndType(ctx, memberID, model.CredPassword)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("get existing credential: %w", err)
	}
	if existing != nil {
		if err := s.credentials.Delete(ctx, existing.ID, memberID); err != nil {
			return fmt.Errorf("delete existing credential: %w", err)
		}
	}

	_, err = s.credentials.Create(ctx, memberID, model.CredPassword, string(hashed), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("create password credential: %w", err)
	}
	return nil
}

func (s *MemberService) HasAny(ctx context.Context) (bool, error) {
	return s.members.HasAny(ctx)
}
