package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ctxKey is an unexported type for context keys defined here, avoiding
// collisions with keys defined in other packages.
type ctxKey int

const userIDCtxKey ctxKey = iota

// UserIDFromContext returns the authenticated user id stored on the request
// context by JWTAuth, or "" if none is present.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDCtxKey).(string); ok {
		return v
	}
	return ""
}

// JWTAuth validates a Bearer token signed with HS256 and rejects anything that
// is missing, malformed, expired, or signed with an unexpected algorithm.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "访问拒绝：缺少 Authorization 请求头"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "访问拒绝：Token 格式错误，必须为 Bearer 格式"})
			c.Abort()
			return
		}
		tokenString := parts[1]

		token, err := jwt.Parse(
			tokenString,
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(secret), nil
			},
			// Only accept HS256 and require an expiry so tokens cannot live
			// forever.
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "访问拒绝：无效或已过期的 Token"})
			c.Abort()
			return
		}

		// Extract a stable user identifier for downstream propagation.
		userID := ""
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			switch v := claims["user_id"].(type) {
			case string:
				userID = v
			case float64:
				userID = fmt.Sprintf("%.0f", v)
			}
			if userID == "" {
				if sub, ok := claims["sub"].(string); ok {
					userID = sub
				}
			}
		}

		if userID != "" {
			c.Set("userID", userID)
			// Also store on the request context so the reverse proxy (which
			// only sees *http.Request) can propagate it downstream.
			ctx := context.WithValue(c.Request.Context(), userIDCtxKey, userID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}
