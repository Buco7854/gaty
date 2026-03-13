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
	Type string `json:"type"`
}

// IssueGateToken creates a signed JWT gate token embedding gateID.
// The token has no expiry — rotation invalidates old tokens via DB comparison.
func IssueGateToken(gateID uuid.UUID, secret []byte) (string, error) {
	claims := gateTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:       uuid.New().String(),
			Subject:  gateID.String(),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
		Type: gateTokenType,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("sign gate token: %w", err)
	}
	return signed, nil
}

// ParseGateToken parses and validates the JWT signature of a gate token,
// returning gateID from the claims.
func ParseGateToken(tokenStr string, secret []byte) (gateID uuid.UUID, err error) {
	var claims gateTokenClaims
	_, err = jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	if claims.Type != gateTokenType {
		return uuid.Nil, ErrInvalidToken
	}
	gateID, err = uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return gateID, nil
}
