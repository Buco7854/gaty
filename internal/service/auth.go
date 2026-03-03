package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
	refreshKeyPrefix = "refresh:"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthService struct {
	users       *repository.UserRepository
	credentials *repository.CredentialRepository
	redis       *redis.Client
	jwtSecret   []byte
}

func NewAuthService(
	users *repository.UserRepository,
	credentials *repository.CredentialRepository,
	redisClient *redis.Client,
	jwtSecret string,
) *AuthService {
	return &AuthService{
		users:       users,
		credentials: credentials,
		redis:       redisClient,
		jwtSecret:   []byte(jwtSecret),
	}
}

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

	_, err = s.credentials.Create(ctx, model.TargetUser, user.ID, model.CredPassword, string(hashed))
	if err != nil {
		return nil, nil, fmt.Errorf("create credential: %w", err)
	}

	tokens, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return tokens, user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, *model.User, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get user: %w", err)
	}

	cred, err := s.credentials.GetByTarget(ctx, model.TargetUser, user.ID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.HashedValue), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return tokens, user, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	userIDStr, err := s.redis.GetDel(ctx, refreshKeyPrefix+refreshToken).Result()
	if err != nil {
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, ErrInvalidToken
	}

	return s.issueTokenPair(ctx, userID)
}

func (s *AuthService) ValidateAccessToken(tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}
	if typ, _ := claims["typ"].(string); typ == "member" {
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

// ValidateMemberToken validates a member JWT and returns the member ID and workspace ID.
func (s *AuthService) ValidateMemberToken(tokenStr string) (memberID, workspaceID uuid.UUID, err error) {
	token, parseErr := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	}, jwt.WithExpirationRequired())
	if parseErr != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	if typ, _ := claims["typ"].(string); typ != "member" {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}

	sub, _ := claims["sub"].(string)
	memberID, err = uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}

	wid, _ := claims["wid"].(string)
	workspaceID, err = uuid.Parse(wid)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}

	return memberID, workspaceID, nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID uuid.UUID) (*TokenPair, error) {
	accessToken, err := s.newAccessToken(userID)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := s.newRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	if err := s.redis.Set(ctx, refreshKeyPrefix+refreshToken, userID.String(), refreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *AuthService) newAccessToken(userID uuid.UUID) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *AuthService) newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
