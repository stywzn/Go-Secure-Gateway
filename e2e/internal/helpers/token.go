// Package helpers builds JWTs for auth tests. By signing tokens itself (using
// the shared secret), the suite can create not just valid tokens but the
// invalid variants the gateway must reject — the interesting half of auth.
package helpers

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ValidToken returns a well-formed HS256 token that expires in one hour.
func ValidToken(secret string, userID any) string {
	return sign(secret, jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
}

// ExpiredToken returns an HS256 token whose exp is already in the past.
func ExpiredToken(secret string) string {
	return sign(secret, jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "u1",
		"exp":     time.Now().Add(-time.Hour).Unix(),
	})
}

// NoExpToken returns a token missing the exp claim (must be rejected).
func NoExpToken(secret string) string {
	return sign(secret, jwt.SigningMethodHS256, jwt.MapClaims{"user_id": "u1"})
}

// WrongSecretToken is validly shaped but signed with the wrong key.
func WrongSecretToken() string {
	return ValidToken("definitely-not-the-real-secret", "u1")
}

// NoneAlgToken forges an unsigned ("alg":"none") token — the classic JWT
// algorithm-confusion attack the gateway must refuse.
func NoneAlgToken() string {
	t := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"user_id": "u1",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	s, _ := t.SignedString(jwt.UnsafeAllowNoneSignatureType)
	return s
}

func sign(secret string, method jwt.SigningMethod, claims jwt.MapClaims) string {
	t := jwt.NewWithClaims(method, claims)
	s, _ := t.SignedString([]byte(secret))
	return s
}
