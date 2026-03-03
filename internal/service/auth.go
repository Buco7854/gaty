package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already taken")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrAlreadyMerged      = errors.New("membership already linked to a user account")
)

const (
	accessTokenTTL   = 15 * time.Minute
	refreshTokenTTL  = 7 * 24 * time.Hour
	refreshKeyPrefix = "refresh:"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthService struct {
	users       *repository.UserRepository
	credentials *repository.CredentialRepository
	memberships *repository.WorkspaceMembershipRepository
	memberCreds *repository.MembershipCredentialRepository
	workspaces  *repository.WorkspaceRepository
	redis       *redis.Client
	jwtSecret   []byte
}

func NewAuthService(
	users *repository.UserRepository,
	credentials *repository.CredentialRepository,
	memberships *repository.WorkspaceMembershipRepository,
	memberCreds *repository.MembershipCredentialRepository,
	workspaces *repository.WorkspaceRepository,
	redisClient *redis.Client,
	jwtSecret string,
) *AuthService {
	return &AuthService{
		users:       users,
		credentials: credentials,
		memberships: memberships,
		memberCreds: memberCreds,
		workspaces:  workspaces,
		redis:       redisClient,
		jwtSecret:   []byte(jwtSecret),
	}
}

// Register creates a new platform user with a password credential and issues a global token pair.
func (s *AuthService) Register(ctx context.Context, email, password string) (*TokenPair, *model.User, error) {
	_, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return nil, nil, ErrEmailTaken
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, nil, fmt.Errorf("check email: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.users.Create(ctx, email)
	if err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	_, err = s.credentials.Create(ctx, user.ID, model.CredPassword, string(hashed), nil, nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create credential: %w", err)
	}

	tokens, err := s.issueGlobalTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return tokens, user, nil
}

// Login authenticates a platform user by email + password and issues a global token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, *model.User, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get user: %w", err)
	}

	cred, err := s.credentials.GetByUserAndType(ctx, user.ID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.HashedValue), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.issueGlobalTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return tokens, user, nil
}

// LoginLocal authenticates a managed member by workspace slug + local username + password.
// Issues a local token pair (sub = membership_id).
func (s *AuthService) LoginLocal(ctx context.Context, workspaceSlug, localUsername, password string) (*TokenPair, *model.WorkspaceMembership, error) {
	ws, err := s.workspaces.GetBySlug(ctx, workspaceSlug)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get workspace: %w", err)
	}

	membership, err := s.memberships.GetByLocalUsername(ctx, ws.ID, localUsername)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get membership: %w", err)
	}

	cred, err := s.memberCreds.GetByMembershipAndType(ctx, membership.ID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get membership credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.HashedValue), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.issueLocalTokenPair(ctx, membership.ID, ws.ID, membership.Role)
	if err != nil {
		return nil, nil, err
	}
	return tokens, membership, nil
}

// Refresh redeems a refresh token and issues a new token pair of the same type.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	val, err := s.redis.GetDel(ctx, refreshKeyPrefix+refreshToken).Result()
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Try to decode as JSON (local token payload).
	var payload map[string]string
	if jsonErr := json.Unmarshal([]byte(val), &payload); jsonErr == nil {
		if payload["type"] == "local" {
			membershipID, err := uuid.Parse(payload["sub"])
			if err != nil {
				return nil, ErrInvalidToken
			}
			workspaceID, err := uuid.Parse(payload["wid"])
			if err != nil {
				return nil, ErrInvalidToken
			}
			role := model.WorkspaceRole(payload["role"])
			return s.issueLocalTokenPair(ctx, membershipID, workspaceID, role)
		}
	}

	// Otherwise treat as raw user_id string (global token).
	userID, err := uuid.Parse(val)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return s.issueGlobalTokenPair(ctx, userID)
}

// Merge links a local membership to the authenticated platform user.
// The user proves ownership of the local account by providing workspace slug + local credentials.
func (s *AuthService) Merge(ctx context.Context, userID uuid.UUID, workspaceSlug, localUsername, password string) error {
	ws, err := s.workspaces.GetBySlug(ctx, workspaceSlug)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}

	membership, err := s.memberships.GetByLocalUsername(ctx, ws.ID, localUsername)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("get membership: %w", err)
	}

	if membership.UserID != nil {
		return ErrAlreadyMerged
	}

	cred, err := s.memberCreds.GetByMembershipAndType(ctx, membership.ID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("get membership credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.HashedValue), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	return s.memberships.MergeUser(ctx, membership.ID, userID)
}

// ValidateAccessToken validates a global (platform user) JWT and returns the user ID.
func (s *AuthService) ValidateAccessToken(tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, s.keyFunc, jwt.WithExpirationRequired())
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}
	if typ, _ := claims["type"].(string); typ != "global" {
		return uuid.Nil, ErrInvalidToken
	}

	sub, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return userID, nil
}

// ValidateMemberToken validates a local (managed member) JWT and returns membership ID, workspace ID, and role.
func (s *AuthService) ValidateMemberToken(tokenStr string) (membershipID, workspaceID uuid.UUID, role model.WorkspaceRole, err error) {
	token, parseErr := jwt.Parse(tokenStr, s.keyFunc, jwt.WithExpirationRequired())
	if parseErr != nil {
		return uuid.Nil, uuid.Nil, "", ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, uuid.Nil, "", ErrInvalidToken
	}
	if typ, _ := claims["type"].(string); typ != "local" {
		return uuid.Nil, uuid.Nil, "", ErrInvalidToken
	}

	sub, _ := claims["sub"].(string)
	membershipID, err = uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", ErrInvalidToken
	}

	wid, _ := claims["wid"].(string)
	workspaceID, err = uuid.Parse(wid)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", ErrInvalidToken
	}

	role = model.WorkspaceRole(claims["role"].(string))
	return membershipID, workspaceID, role, nil
}

// IssueLocalTokenPair issues a local JWT pair for a managed member.
// Called by SSOService after a successful SSO callback.
func (s *AuthService) IssueLocalTokenPair(ctx context.Context, membershipID, workspaceID uuid.UUID, role model.WorkspaceRole) (*TokenPair, error) {
	return s.issueLocalTokenPair(ctx, membershipID, workspaceID, role)
}

func (s *AuthService) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method")
	}
	return s.jwtSecret, nil
}

func (s *AuthService) issueGlobalTokenPair(ctx context.Context, userID uuid.UUID) (*TokenPair, error) {
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID.String(),
		"type": "global",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(accessTokenTTL).Unix(),
	}).SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	if err := s.redis.Set(ctx, refreshKeyPrefix+refreshToken, userID.String(), refreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *AuthService) issueLocalTokenPair(ctx context.Context, membershipID, workspaceID uuid.UUID, role model.WorkspaceRole) (*TokenPair, error) {
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  membershipID.String(),
		"type": "local",
		"wid":  workspaceID.String(),
		"role": string(role),
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(accessTokenTTL).Unix(),
	}).SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("sign local access token: %w", err)
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"type": "local",
		"sub":  membershipID.String(),
		"wid":  workspaceID.String(),
		"role": string(role),
	})
	if err := s.redis.Set(ctx, refreshKeyPrefix+refreshToken, string(payload), refreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
