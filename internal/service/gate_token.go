package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const gateTokenType = "gate_token"

type gateTokenClaims struct {
	jwt.RegisteredClaims
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Type        string    `json:"type"`
}

// IssueGateToken creates a signed JWT gate token embedding gateID and workspaceID.
// The token has no expiry — rotation (SetToken + new JWT) invalidates old tokens via DB comparison.
func IssueGateToken(gateID, workspaceID uuid.UUID, secret []byte) (string, error) {
	claims := gateTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:       uuid.New().String(),
			Subject:  gateID.String(),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
		WorkspaceID: workspaceID,
		Type:        gateTokenType,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("sign gate token: %w", err)
	}
	return signed, nil
}

// ParseGateToken parses and validates the JWT signature of a gate token,
// returning gateID and workspaceID from the claims.
//
// This only verifies the cryptographic signature — it does NOT check whether
// the token is still current. Call gateRepo.GetByToken to confirm it has not
// been rotated (DB comparison).
func ParseGateToken(tokenStr string, secret []byte) (gateID, workspaceID uuid.UUID, err error) {
	var claims gateTokenClaims
	_, err = jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	if claims.Type != gateTokenType {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	gateID, err = uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	return gateID, claims.WorkspaceID, nil
}
