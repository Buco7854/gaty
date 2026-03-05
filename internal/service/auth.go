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
	users                 repository.UserRepository
	credentials           repository.CredentialRepository
	memberships           repository.WorkspaceMembershipRepository
	memberCreds           repository.MembershipCredentialRepository
	workspaces            repository.WorkspaceRepository
	redis                 *redis.Client
	jwtSecret             []byte
	globalSessionDuration time.Duration // 0 = infinite
}

func NewAuthService(
	users repository.UserRepository,
	credentials repository.CredentialRepository,
	memberships repository.WorkspaceMembershipRepository,
	memberCreds repository.MembershipCredentialRepository,
	workspaces repository.WorkspaceRepository,
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

// LoginLocal authenticates a managed member by workspace ID + local username + password.
// Issues a local token pair (sub = membership_id) with a session duration resolved from
// the member's auth_config, falling back to the workspace default, then to defaultSessionTTL.
func (s *AuthService) LoginLocal(ctx context.Context, workspaceID uuid.UUID, localUsername, password string) (*TokenPair, *model.WorkspaceMembership, error) {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
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

	sessionDuration := resolveSessionDuration(ws.MemberAuthConfig)
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
		typ, _ := payload["type"].(string)

		switch typ {
		case "local":
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

		case "global":
			userID, err := uuid.Parse(payload["sub"].(string))
			if err != nil {
				return nil, ErrInvalidToken
			}
			return s.issueGlobalTokenPair(ctx, userID, sessionDuration)

		case "pin_session":
			pinID, err := uuid.Parse(payload["sub"].(string))
			if err != nil {
				return nil, ErrInvalidToken
			}
			gateID, err := uuid.Parse(payload["gate_id"].(string))
			if err != nil {
				return nil, ErrInvalidToken
			}
			var perms []string
			if raw, ok := payload["permissions"].([]interface{}); ok {
				for _, v := range raw {
					if s, ok := v.(string); ok {
						perms = append(perms, s)
					}
				}
			}
			return s.IssueGatePinSession(ctx, pinID, gateID, sessionDuration, perms)
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
// The user proves ownership of the local account by providing workspace ID + local credentials.
func (s *AuthService) Merge(ctx context.Context, userID uuid.UUID, workspaceID uuid.UUID, localUsername, password string) error {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
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

// IssueGatePinSession issues a short-lived JWT for a PIN session.
// sub = pin_id, type = "pin_session", gate_id and permissions embedded in claims.
// permissions should never include "gate:manage".
func (s *AuthService) IssueGatePinSession(ctx context.Context, pinID, gateID uuid.UUID, sessionDuration time.Duration, permissions []string) (*TokenPair, error) {
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":         pinID.String(),
		"type":        "pin_session",
		"gate_id":     gateID.String(),
		"permissions": permissions,
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(accessTokenTTL).Unix(),
	}).SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("sign pin session access token: %w", err)
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate pin session refresh token: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"type":             "pin_session",
		"sub":              pinID.String(),
		"gate_id":          gateID.String(),
		"permissions":      permissions,
		"session_duration": sessionDuration.Seconds(),
	})
	if err := s.redis.Set(ctx, refreshKeyPrefix+refreshToken, string(payload), sessionDuration).Err(); err != nil {
		return nil, fmt.Errorf("store pin session refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// ValidatePinSessionToken validates a pin_session JWT and returns pin ID, gate ID, and permissions.
func (s *AuthService) ValidatePinSessionToken(tokenStr string) (pinID, gateID uuid.UUID, permissions []string, err error) {
	token, parseErr := jwt.Parse(tokenStr, s.keyFunc, jwt.WithExpirationRequired())
	if parseErr != nil {
		return uuid.Nil, uuid.Nil, nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, uuid.Nil, nil, ErrInvalidToken
	}
	if typ, _ := claims["type"].(string); typ != "pin_session" {
		return uuid.Nil, uuid.Nil, nil, ErrInvalidToken
	}

	pinID, err = uuid.Parse(claims["sub"].(string))
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, ErrInvalidToken
	}
	gateID, err = uuid.Parse(claims["gate_id"].(string))
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, ErrInvalidToken
	}

	if raw, ok := claims["permissions"].([]interface{}); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				permissions = append(permissions, s)
			}
		}
	}
	return pinID, gateID, permissions, nil
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
	sessionDuration := resolveSessionDuration(ws.MemberAuthConfig)
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

// resolveSessionDuration reads session_duration (in seconds) from the workspace-level
// member_auth_config. This is intentionally workspace-global — per-member overrides are
// not supported so that admins have a single place to control session lifetime.
// A value of 0 means infinite (no expiry).
func resolveSessionDuration(workspaceAuthConfig map[string]any) time.Duration {
	if workspaceAuthConfig != nil {
		v, ok := workspaceAuthConfig["session_duration"]
		if ok {
			if secs, ok := v.(float64); ok {
				if secs == 0 {
					return 0 // infinite
				}
				if secs > 0 {
					return time.Duration(secs) * time.Second
				}
			}
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
