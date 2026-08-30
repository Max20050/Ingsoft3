package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned when a JWT is malformed, expired, or signed with the wrong key.
var ErrInvalidToken = errors.New("invalid or expired token")

const tokenTTL = 24 * time.Hour

type claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// TokenIssuer signs and verifies JWTs for authenticated sessions.
type TokenIssuer struct {
	secret []byte
}

func NewTokenIssuer(secret string) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret)}
}

// IssueToken creates a signed JWT carrying the given user ID, valid for tokenTTL.
func (i *TokenIssuer) IssueToken(userID string) (string, error) {
	now := time.Now()
	c := claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(i.secret)
}

// VerifyToken parses and validates a JWT, returning the embedded user ID.
func (i *TokenIssuer) VerifyToken(tokenString string) (string, error) {
	var c claims
	token, err := jwt.ParseWithClaims(tokenString, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return i.secret, nil
	})
	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}
	return c.UserID, nil
}
