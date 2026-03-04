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
	accessTokenTTL      = 15 * time.Minute
	defaultSessionTTL   = 7 * 24 * time.Hour
	refreshKeyPrefix    = "refresh:"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthService struct {
	users                 *repository.UserRepository
	credentials           *repository.CredentialRepository
	memberships           *repository.WorkspaceMembershipRepository
	memberCreds           *repository.MembershipCredentialRepository
	workspaces            *repository.WorkspaceRepository
	redis                 *redis.Client
	jwtSecret             []byte
	globalSessionDuration time.Duration // 0 = infinite
}

func NewAuthService(
	users *repository.UserRepository,
	credentials *repository.CredentialRepository,
	memberships *repository.WorkspaceMembershipRepository,
	memberCreds *repository.MembershipCredentialRepository,
	workspaces *repository.WorkspaceRepository,
	redisClient *redis.Client,
	jwtSecret string,
	globalSessionDuration time.Duration,
) *AuthService {
	return &AuthService{
		users:                 users,
		credentials:           credentials,
		memberships:           memberships,
		memberCreds:           memberCreds,
		workspaces:            workspaces,
		redis:                 redisClient,
		jwtSecret:             []byte(jwtSecret),
		globalSessionDuration: globalSessionDuration,
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

	tokens, err := s.issueGlobalTokenPair(ctx, user.ID, s.globalSessionDuration)
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

	tokens, err := s.issueGlobalTokenPair(ctx, user.ID, s.globalSessionDuration)
	if err != nil {
		return nil, nil, err
	}
	return tokens, user, nil
}

// LoginLocal authenticates a managed member by workspace slug + local username + password.
// Issues a local token pair (sub = membership_id) with a session duration resolved from
// the member's auth_config, falling back to the workspace default, then to defaultSessionTTL.
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

	sessionDuration := resolveSessionDuration(membership.AuthConfig, ws.MemberAuthConfig)
	tokens, err := s.issueLocalTokenPair(ctx, membership.ID, ws.ID, membership.Role, sessionDuration)
	if err != nil {
		return nil, nil, err
	}
	return tokens, membership, nil
}

// Refresh redeems a refresh token and issues a new token pair of the same type,
// preserving the original session duration.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	val, err := s.redis.GetDel(ctx, refreshKeyPrefix+refreshToken).Result()
	if err != nil {
		return nil, ErrInvalidToken
	}

	var payload map[string]any
	if jsonErr := json.Unmarshal([]byte(val), &payload); jsonErr == nil {
		sessionDuration := payloadSessionDuration(payload)

		if typ, _ := payload["type"].(string); typ == "local" {
			membershipID, err := uuid.Parse(payload["sub"].(string))
			if err != nil {
				return nil, ErrInvalidToken
			}
			workspaceID, err := uuid.Parse(payload["wid"].(string))
			if err != nil {
				return nil, ErrInvalidToken
			}
			role := model.WorkspaceRole(payload["role"].(string))
			return s.issueLocalTokenPair(ctx, membershipID, workspaceID, role, sessionDuration)
		}

		if typ, _ := payload["type"].(string); typ == "global" {
			sub, _ := payload["sub"].(string)
			userID, err := uuid.Parse(sub)
			if err != nil {
				return nil, ErrInvalidToken
			}
			return s.issueGlobalTokenPair(ctx, userID, sessionDuration)
		}
	}

	// Backward compat: plain UUID string for old global tokens.
	userID, err := uuid.Parse(val)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return s.issueGlobalTokenPair(ctx, userID, s.globalSessionDuration)
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
// Session duration is resolved from the member's auth_config and workspace defaults.
func (s *AuthService) IssueLocalTokenPair(ctx context.Context, membershipID, workspaceID uuid.UUID, role model.WorkspaceRole) (*TokenPair, error) {
	membership, err := s.memberships.GetByID(ctx, membershipID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get membership: %w", err)
	}
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	sessionDuration := resolveSessionDuration(membership.AuthConfig, ws.MemberAuthConfig)
	return s.issueLocalTokenPair(ctx, membershipID, workspaceID, role, sessionDuration)
}

func (s *AuthService) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method")
	}
	return s.jwtSecret, nil
}

func (s *AuthService) issueGlobalTokenPair(ctx context.Context, userID uuid.UUID, sessionDuration time.Duration) (*TokenPair, error) {
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

	payload, _ := json.Marshal(map[string]any{
		"type":             "global",
		"sub":              userID.String(),
		"session_duration": sessionDuration.Seconds(),
	})
	if err := s.redis.Set(ctx, refreshKeyPrefix+refreshToken, string(payload), sessionDuration).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *AuthService) issueLocalTokenPair(ctx context.Context, membershipID, workspaceID uuid.UUID, role model.WorkspaceRole, sessionDuration time.Duration) (*TokenPair, error) {
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

	payload, _ := json.Marshal(map[string]any{
		"type":             "local",
		"sub":              membershipID.String(),
		"wid":              workspaceID.String(),
		"role":             string(role),
		"session_duration": sessionDuration.Seconds(),
	})
	if err := s.redis.Set(ctx, refreshKeyPrefix+refreshToken, string(payload), sessionDuration).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// resolveSessionDuration reads session_duration (in seconds) from member-level auth_config,
// falling back to the workspace member_auth_config default, then to defaultSessionTTL.
// A value of 0 means infinite (no expiry).
func resolveSessionDuration(memberAuthConfig, workspaceAuthConfig map[string]any) time.Duration {
	for _, cfg := range []map[string]any{memberAuthConfig, workspaceAuthConfig} {
		if cfg == nil {
			continue
		}
		v, ok := cfg["session_duration"]
		if !ok {
			continue
		}
		secs, ok := v.(float64) // JSON numbers unmarshal as float64
		if !ok {
			continue
		}
		if secs == 0 {
			return 0 // infinite
		}
		if secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return defaultSessionTTL
}

// payloadSessionDuration extracts the session_duration from a stored Redis payload.
func payloadSessionDuration(payload map[string]any) time.Duration {
	v, ok := payload["session_duration"]
	if !ok {
		return defaultSessionTTL
	}
	secs, ok := v.(float64)
	if !ok {
		return defaultSessionTTL
	}
	if secs == 0 {
		return 0 // infinite
	}
	if secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return defaultSessionTTL
}

func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
