package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func signHS256(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func runAuth(t *testing.T, header string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	r := gin.New()
	var gotUserID string
	r.Use(JWTAuth(testSecret))
	r.GET("/", func(c *gin.Context) {
		gotUserID = UserIDFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	r.ServeHTTP(w, req)
	return w, gotUserID
}

func TestJWTAuth_ValidTokenPassesAndPropagatesUser(t *testing.T) {
	tok := signHS256(t, jwt.MapClaims{
		"user_id": float64(9527),
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	w, userID := runAuth(t, "Bearer "+tok)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if userID != "9527" {
		t.Errorf("user id not propagated: got %q, want 9527", userID)
	}
}

func TestJWTAuth_Rejections(t *testing.T) {
	expired := signHS256(t, jwt.MapClaims{
		"user_id": "u1",
		"exp":     time.Now().Add(-time.Hour).Unix(),
	})
	noExp := signHS256(t, jwt.MapClaims{"user_id": "u1"})

	// Token signed with a different secret.
	wrongTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "u1",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	wrongSig, _ := wrongTok.SignedString([]byte("other-secret"))

	cases := map[string]string{
		"no header":       "",
		"wrong scheme":    "Basic abc",
		"garbage token":   "Bearer not.a.jwt",
		"expired":         "Bearer " + expired,
		"missing exp":     "Bearer " + noExp,
		"wrong signature": "Bearer " + wrongSig,
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			w, _ := runAuth(t, header)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 for %q, got %d", name, w.Code)
			}
		})
	}
}
