package enroll

import (
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
}

// IssueToken returns a signed JWT with the given ttl.
func IssueToken(secret []byte, ttl time.Duration) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("empty jwt secret")
	}
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "enroll-" + now.Format("20060102T150405"),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// VerifyToken validates token signature and expiry.
func VerifyToken(secret []byte, tokenStr string) (*Claims, error) {
	if len(secret) == 0 {
		return nil, errors.New("empty jwt secret")
	}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	tok, err := parser.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}
	return claims, nil
}
