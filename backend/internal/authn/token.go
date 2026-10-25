// Package authn holds transport-level authentication: access-token signing and
// verification, the authenticated user in the context, and the gRPC interceptor
// that enforces them. It owns no domain rules and no database access.
package authn

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Sign mints a short-lived HS256 access token for subject.
func Sign(secret string, subject pgtype.UUID, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   subject.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Verify validates an access token and returns its subject. The key func accepts
// only HMAC methods, so an attacker cannot swap the algorithm.
func Verify(secret, tokenStr string) (pgtype.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return pgtype.UUID{}, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return pgtype.UUID{}, errors.New("invalid token")
	}

	var id pgtype.UUID
	if err := id.Scan(claims.Subject); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}
