package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"poker-fate-server/internal/ws"
)

func (r *Router) SignMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "read body failed"})
			c.Abort()
			return
		}

		if len(body) == 0 {
			c.Set("body", map[string]interface{}{})
			c.Next()
			return
		}

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid json"})
			c.Abort()
			return
		}

		random, _ := req["random"].(string)
		ts, _ := req["ts"].(string)
		sign, _ := req["sign"].(string)

		if sign != "" && !ws.VerifySign(random, ts, sign, r.Config.Auth.HTTPSalt) {
			r.Logger.Warn("middleware: invalid sign",
				zap.String("path", c.Request.URL.Path),
				zap.String("random", random),
				zap.String("ts", ts),
				zap.String("sign", sign))
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid sign"})
			c.Abort()
			return
		}

		c.Set("body", req)
		c.Next()
	}
}

func (r *Router) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			auth = c.GetHeader("authorization")
		}

		if auth == "" {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "missing authorization"})
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		tokenStr = strings.TrimSpace(tokenStr)

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(r.Config.Auth.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid claims"})
			c.Abort()
			return
		}

		uid, _ := claims["uid"].(float64)

		stored, err := r.Redis.Get(context.Background(), fmt.Sprintf("session:%d", int64(uid))).Result()
		if err != nil || stored != tokenStr {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "token expired"})
			c.Abort()
			return
		}

		c.Set("uid", int64(uid))
		c.Next()
	}
}
