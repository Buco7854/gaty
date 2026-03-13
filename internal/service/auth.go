package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrWeakPassword       = errors.New("password must contain at least one uppercase letter, one lowercase letter, and one digit")
)

const (
	AccessTokenTTL    = 15 * time.Minute
	defaultSessionTTL = 7 * 24 * time.Hour
	refreshKeyPrefix  = "refresh:"
)

type TokenPair struct {
	AccessToken     string
	RefreshToken    string
	SessionDuration time.Duration
	AccessTokenTTL  time.Duration
}

// RefreshResult carries the new tokens plus session metadata.
type RefreshResult struct {
	Tokens      *TokenPair
	Type        string         // "member", "pin_session"
	Member      *model.Member  // member
	GateID      uuid.UUID      // pin_session
	Permissions []string       // pin_session
}

// PasswordPolicy configures password complexity requirements.
type PasswordPolicy struct {
	MinLength    int
	RequireUpper bool
	RequireLower bool
	RequireDigit bool
}

type AuthService struct {
	members        repository.MemberRepository
	credentials    repository.CredentialRepository
	redis          *redis.Client
	jwtSecret      []byte
	sessionDuration time.Duration // 0 = infinite
	passwordPolicy PasswordPolicy
}

func NewAuthService(
	members repository.MemberRepository,
	credentials repository.CredentialRepository,
	redisClient *redis.Client,
	jwtSecret string,
	sessionDuration time.Duration,
	passwordPolicy PasswordPolicy,
) *AuthService {
	return &AuthService{
		members:         members,
		credentials:     credentials,
		redis:           redisClient,
		jwtSecret:       []byte(jwtSecret),
		sessionDuration: sessionDuration,
		passwordPolicy:  passwordPolicy,
	}
}

func (s *AuthService) validatePassword(password string) error {
	if len(password) < s.passwordPolicy.MinLength {
		return fmt.Errorf("%w: minimum %d characters", ErrWeakPassword, s.passwordPolicy.MinLength)
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if s.passwordPolicy.RequireUpper && !hasUpper {
		return fmt.Errorf("%w: must contain an uppercase letter", ErrWeakPassword)
	}
	if s.passwordPolicy.RequireLower && !hasLower {
		return fmt.Errorf("%w: must contain a lowercase letter", ErrWeakPassword)
	}
	if s.passwordPolicy.RequireDigit && !hasDigit {
		return fmt.Errorf("%w: must contain a digit", ErrWeakPassword)
	}
	return nil
}

// Register creates a new member with a password credential and issues a token pair.
func (s *AuthService) Register(ctx context.Context, username, password string, displayName *string) (*TokenPair, *model.Member, error) {
	if err := s.validatePassword(password); err != nil {
		return nil, nil, err
	}

	_, err := s.members.GetByUsername(ctx, username)
	if err == nil {
		return nil, nil, ErrUsernameTaken
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, nil, fmt.Errorf("check username: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	member, err := s.members.Create(ctx, username, displayName, model.RoleMember)
	if err != nil {
		return nil, nil, fmt.Errorf("create member: %w", err)
	}

	_, err = s.credentials.Create(ctx, member.ID, model.CredPassword, string(hashed), nil, nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create credential: %w", err)
	}

	tokens, err := s.issueTokenPair(ctx, member.ID, member.Role, s.sessionDuration)
	if err != nil {
		return nil, nil, err
	}
	return tokens, member, nil
}

// Login authenticates a member by username + password and issues a token pair.
func (s *AuthService) Login(ctx context.Context, username, password string) (*TokenPair, *model.Member, error) {
	member, err := s.members.GetByUsername(ctx, username)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get member: %w", err)
	}

	cred, err := s.credentials.GetByMemberAndType(ctx, member.ID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.HashedValue), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.issueTokenPair(ctx, member.ID, member.Role, s.sessionDuration)
	if err != nil {
		return nil, nil, err
	}
	return tokens, member, nil
}

// Refresh redeems a refresh token and issues a new token pair.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*RefreshResult, error) {
	val, err := s.redis.GetDel(ctx, refreshKey(refreshToken)).Result()
	if err != nil {
		return nil, ErrInvalidToken
	}

	var payload map[string]any
	if jsonErr := json.Unmarshal([]byte(val), &payload); jsonErr == nil {
		sessionDuration := payloadSessionDuration(payload)
		typ, _ := payload["type"].(string)

		switch typ {
		case "member":
			sub, ok := payload["sub"].(string)
			if !ok {
				return nil, ErrInvalidToken
			}
			memberID, err := uuid.Parse(sub)
			if err != nil {
				return nil, ErrInvalidToken
			}
			member, err := s.members.GetByID(ctx, memberID)
			if err != nil {
				return nil, ErrInvalidToken
			}
			tokens, err := s.issueTokenPair(ctx, memberID, member.Role, sessionDuration)
			if err != nil {
				return nil, err
			}
			return &RefreshResult{Tokens: tokens, Type: "member", Member: member}, nil

		case "pin_session":
			sub, ok := payload["sub"].(string)
			if !ok {
				return nil, ErrInvalidToken
			}
			pinID, err := uuid.Parse(sub)
			if err != nil {
				return nil, ErrInvalidToken
			}
			gateIDStr, ok := payload["gate_id"].(string)
			if !ok {
				return nil, ErrInvalidToken
			}
			gateID, err := uuid.Parse(gateIDStr)
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
			tokens, err := s.IssueGatePinSession(ctx, pinID, gateID, sessionDuration, perms)
			if err != nil {
				return nil, err
			}
			return &RefreshResult{Tokens: tokens, Type: "pin_session", GateID: gateID, Permissions: perms}, nil
		}
	}

	return nil, ErrInvalidToken
}

// RevokeRefreshToken deletes a refresh token from Redis.
func (s *AuthService) RevokeRefreshToken(ctx context.Context, token string) {
	s.redis.Del(ctx, refreshKey(token))
}

// ValidateAccessToken validates a member JWT and returns member ID and role.
func (s *AuthService) ValidateAccessToken(tokenStr string) (memberID uuid.UUID, role model.Role, err error) {
	token, parseErr := jwt.Parse(tokenStr, s.keyFunc, jwt.WithExpirationRequired())
	if parseErr != nil {
		return uuid.Nil, "", ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, "", ErrInvalidToken
	}
	if typ, _ := claims["type"].(string); typ != "member" {
		return uuid.Nil, "", ErrInvalidToken
	}

	sub, _ := claims["sub"].(string)
	memberID, err = uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, "", ErrInvalidToken
	}

	roleStr, ok := claims["role"].(string)
	if !ok || roleStr == "" {
		return uuid.Nil, "", ErrInvalidToken
	}
	role = model.Role(roleStr)
	return memberID, role, nil
}

// IssueGatePinSession issues a short-lived JWT for a PIN session.
func (s *AuthService) IssueGatePinSession(ctx context.Context, pinID, gateID uuid.UUID, sessionDuration time.Duration, permissions []string) (*TokenPair, error) {
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":         pinID.String(),
		"type":        "pin_session",
		"gate_id":     gateID.String(),
		"permissions": permissions,
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(AccessTokenTTL).Unix(),
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
	if err := s.redis.Set(ctx, refreshKey(refreshToken), string(payload), sessionDuration).Err(); err != nil {
		return nil, fmt.Errorf("store pin session refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, AccessTokenTTL: AccessTokenTTL}, nil
}

// ValidatePinSessionToken validates a pin_session JWT.
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

// IssueTokenPair issues a member JWT pair (exposed for SSO callback).
func (s *AuthService) IssueTokenPair(ctx context.Context, memberID uuid.UUID, role model.Role) (*TokenPair, error) {
	return s.issueTokenPair(ctx, memberID, role, s.sessionDuration)
}

func (s *AuthService) keyFunc(t *jwt.Token) (any, error) {
	if t.Method != jwt.SigningMethodHS256 {
		return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
	}
	return s.jwtSecret, nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, memberID uuid.UUID, role model.Role, sessionDuration time.Duration) (*TokenPair, error) {
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  memberID.String(),
		"type": "member",
		"role": string(role),
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(AccessTokenTTL).Unix(),
	}).SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"type":             "member",
		"sub":              memberID.String(),
		"role":             string(role),
		"session_duration": sessionDuration.Seconds(),
	})
	if err := s.redis.Set(ctx, refreshKey(refreshToken), string(payload), sessionDuration).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, SessionDuration: sessionDuration, AccessTokenTTL: AccessTokenTTL}, nil
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
		return 0
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

func refreshKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return refreshKeyPrefix + hex.EncodeToString(h[:])
}
