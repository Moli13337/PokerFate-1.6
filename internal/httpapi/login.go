package httpapi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"poker-fate-server/internal/ws"
)

func (r *Router) LoginHandler(c *gin.Context) {
	req, _ := c.Get("body")
	params, ok := req.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid params"})
		return
	}

	loginType := intVal(params, "type")
	token := strVal(params, "token")
	osType := strVal(params, "os")
	imei := strVal(params, "imei")
	lang := strVal(params, "lang")
	verify := strVal(params, "verify")

	switch loginType {
	case 1:
		if !ws.VerifyGuest(osType, imei, verify, r.Config.Auth.GuestSalt) {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "verify failed"})
			return
		}
	case 2:
		password := strVal(params, "verify")
		_ = password
	default:
	}

	var uid int64
	var isReg bool
	var userEmail string
	var isGuest bool

	if loginType == 1 || loginType == 5 || loginType == 6 {
		identifier := imei
		if identifier == "" {
			identifier = token
		}
		uid, isReg, _ = r.findOrCreateUser(identifier, "", loginType, osType, imei, lang)
		isGuest = true
	} else if loginType == 2 {
		email := token
		password := verify
		uid, isReg, _ = r.findOrCreateUser("", email, loginType, osType, imei, lang)
		if !isReg {
			var storedPwd string
			r.DB.QueryRowContext(context.Background(),
				`SELECT password FROM users WHERE uid=$1`, uid).Scan(&storedPwd)
			hashedPwd := ws.DoubleMD5(password)
			if storedPwd == "" {
				// First login with password - set it
				r.DB.ExecContext(context.Background(),
					`UPDATE users SET password=$1 WHERE uid=$2`, hashedPwd, uid)
			} else if storedPwd != hashedPwd {
				c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "wrong password"})
				return
			}
		} else {
			// New registration - save the password
			r.DB.ExecContext(context.Background(),
				`UPDATE users SET password=$1 WHERE uid=$2`, ws.DoubleMD5(password), uid)
		}
		userEmail = email
		isGuest = false
	} else if loginType == 3 {
		uid, isReg, _ = r.findOrCreateUser("tw_"+token, "", loginType, osType, imei, lang)
	} else {
		uid, isReg, _ = r.findOrCreateUser(token, "", loginType, osType, imei, lang)
	}

	if uid == 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "login failed"})
		return
	}

	r.DB.ExecContext(context.Background(),
		`UPDATE users SET login_time=NOW(), login_ip=$1 WHERE uid=$2`,
		c.ClientIP(), uid)

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid": uid,
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	})
	tokenStr, _ := jwtToken.SignedString([]byte(r.Config.Auth.JWTSecret))

	r.Redis.Set(context.Background(), fmt.Sprintf("session:%d", uid), tokenStr, 7*24*time.Hour)

	rdkeyBytes := md5.Sum([]byte(uuid.New().String()))
	rdkey := hex.EncodeToString(rdkeyBytes[:])
	r.Redis.Set(context.Background(), fmt.Sprintf("rdkey:%d", uid), rdkey, 10*time.Minute)

	bindEmail := ""
	if userEmail != "" {
		bindEmail = userEmail
	}

	c.JSON(http.StatusOK, gin.H{
		"code":          0,
		"uid":           uid,
		"rdkey":         rdkey,
		"authorization": tokenStr,
		"bind_email":    bindEmail,
		"reg_chnl":      2,
		"update_white":  true,
		"event_white":   true,
		"login_region":  "",
		"is_del":        false,
		"able_pay":      true,
		"login_ip":      c.ClientIP(),
		"is_guest":      isGuest,
		"stove_guid":    "",
		"is_reg":        isReg,
		"server": gin.H{
			"server": []gin.H{
				{
					"http_host":   fmt.Sprintf("http://%s%s/", r.Config.Server.Host, r.Config.Server.HTTPAddr),
					"server_host": fmt.Sprintf("ws://%s%s", r.Config.Server.Host, r.Config.Server.WSAddr),
				},
			},
		},
	})
}

func (r *Router) EmailRegisterHandler(c *gin.Context) {
	req, _ := c.Get("body")
	params, ok := req.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid params"})
		return
	}

	email := strVal(params, "email")
	password := strVal(params, "password")
	osType := strVal(params, "os")
	imei := strVal(params, "imei")
	lang := strVal(params, "lang")

	if email == "" {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "email required"})
		return
	}

	var existUID int64
	r.DB.QueryRowContext(context.Background(),
		`SELECT uid FROM users WHERE email=$1`, email).Scan(&existUID)
	if existUID > 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "email already registered"})
		return
	}

	uid, _, err := r.findOrCreateUser("", email, 2, osType, imei, lang)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "register failed"})
		return
	}

	if password != "" {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET password=$1 WHERE uid=$2`, ws.DoubleMD5(password), uid)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "uid": uid})
}

func (r *Router) CaptchaHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (r *Router) ForgotPasswordHandler(c *gin.Context) {
	req, _ := c.Get("body")
	params, ok := req.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": -1})
		return
	}

	email := strVal(params, "email")
	password := strVal(params, "password")

	if email != "" && password != "" {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET password=$1 WHERE email=$2`, ws.DoubleMD5(password), email)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (r *Router) XOauthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": -1})
}

func (r *Router) CheckServerHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (r *Router) findOrCreateUser(identifier, email string, loginType int, osType, imei, lang string) (int64, bool, error) {
	var uid int64

	if email != "" {
		err := r.DB.QueryRowContext(context.Background(),
			`SELECT uid FROM users WHERE email=$1`, email).Scan(&uid)
		if err == nil {
			return uid, false, nil
		}
	}

	if identifier != "" && email == "" {
		err := r.DB.QueryRowContext(context.Background(),
			`SELECT uid FROM users WHERE imei=$1 AND login_type=$2`, identifier, loginType).Scan(&uid)
		if err == nil {
			return uid, false, nil
		}
	}

	var maxUID int64
	r.DB.QueryRowContext(context.Background(),
		`SELECT COALESCE(MAX(uid), 10000) FROM users`).Scan(&maxUID)
	newUID := maxUID + 1

	_, err := r.DB.ExecContext(context.Background(),
		`INSERT INTO users (uid, name, token, login_type, os, imei, email, password, gold, lang, chnl)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, '', $8, $9, 2)`,
		newUID, fmt.Sprintf("Player%d", newUID), uuid.New().String(), loginType, osType, identifier, email,
		r.Config.Game.InitialGold, lang)
	if err != nil {
		return 0, true, err
	}

	// Private server: unlock all characters, skins, items and send a welcome mail.
	ws.InitNewAccount(r.DB, newUID)

	return newUID, true, nil
}

func strVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func intVal(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
