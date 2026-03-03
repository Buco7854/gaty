package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrUsernameTaken = errors.New("username already taken in this workspace")

type MemberService struct {
	members     *repository.MemberRepository
	credentials *repository.CredentialRepository
	jwtSecret   []byte
}

func NewMemberService(
	members *repository.MemberRepository,
	credentials *repository.CredentialRepository,
	jwtSecret string,
) *MemberService {
	return &MemberService{
		members:     members,
		credentials: credentials,
		jwtSecret:   []byte(jwtSecret),
	}
}

func (s *MemberService) Create(ctx context.Context, workspaceID uuid.UUID, displayName string, email *string, username, password string) (*model.Member, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	member, err := s.members.Create(ctx, workspaceID, displayName, email, username)
	if errors.Is(err, repository.ErrAlreadyExists) {
		return nil, ErrUsernameTaken
	}
	if err != nil {
		return nil, fmt.Errorf("create member: %w", err)
	}

	_, err = s.credentials.Create(ctx, model.TargetMember, member.ID, model.CredPassword, string(hashed))
	if err != nil {
		return nil, fmt.Errorf("create member credential: %w", err)
	}

	return member, nil
}

// Login authenticates a member by username OR email within a workspace.
// Returns an access token and the member on success.
func (s *MemberService) Login(ctx context.Context, workspaceID uuid.UUID, login, password string) (string, *model.Member, error) {
	member, err := s.members.GetByUsernameOrEmail(ctx, workspaceID, login)
	if errors.Is(err, repository.ErrNotFound) {
		return "", nil, ErrInvalidCredentials
	}
	if err != nil {
		return "", nil, fmt.Errorf("get member: %w", err)
	}

	cred, err := s.credentials.GetByTarget(ctx, model.TargetMember, member.ID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return "", nil, ErrInvalidCredentials
	}
	if err != nil {
		return "", nil, fmt.Errorf("get member credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.HashedValue), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err := s.issueMemberToken(member.ID, workspaceID)
	if err != nil {
		return "", nil, err
	}
	return token, member, nil
}

func (s *MemberService) GetByID(ctx context.Context, memberID, workspaceID uuid.UUID) (*model.Member, error) {
	return s.members.GetByID(ctx, memberID, workspaceID)
}

func (s *MemberService) List(ctx context.Context, workspaceID uuid.UUID) ([]*model.Member, error) {
	return s.members.List(ctx, workspaceID)
}

func (s *MemberService) Update(ctx context.Context, memberID, workspaceID uuid.UUID, displayName string, email *string) (*model.Member, error) {
	return s.members.Update(ctx, memberID, workspaceID, displayName, email)
}

func (s *MemberService) Delete(ctx context.Context, memberID, workspaceID uuid.UUID) error {
	return s.members.SoftDelete(ctx, memberID, workspaceID)
}

func (s *MemberService) issueMemberToken(memberID, workspaceID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": memberID.String(),
		"wid": workspaceID.String(),
		"typ": "member",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(accessTokenTTL).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign member token: %w", err)
	}
	return token, nil
}
