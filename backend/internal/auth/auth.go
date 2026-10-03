// Package auth 提供密码哈希与 JWT 签发/校验。
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userContextKey contextKey = "user"

// Claims JWT 载荷
type Claims struct {
	UserID int64  `json:"uid"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	UnitNo string `json:"unit_no"`
	jwt.RegisteredClaims
}

// User 注入请求上下文的当前用户
type User struct {
	ID     int64
	Name   string
	Role   string
	UnitNo string
}

func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

func Issue(secret string, c Claims) (string, error) {
	c.RegisteredClaims = jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour))}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString([]byte(secret))
}

func Parse(secret, tokenStr string) (*Claims, error) {
	c := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

// Middleware 校验 Bearer Token
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}
			claims, err := Parse(secret, strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, &User{
				ID: claims.UserID, Name: claims.Name, Role: claims.Role, UnitNo: claims.UnitNo,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CurrentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userContextKey).(*User)
	return u
}

// Require 要求已登录，返回 false 时已写出 401
func Require(w http.ResponseWriter, r *http.Request) *User {
	u := CurrentUser(r)
	if u == nil {
		http.Error(w, `{"error":"未登录"}`, http.StatusUnauthorized)
		return nil
	}
	return u
}

// RequireRole 要求特定角色之一
func RequireRole(w http.ResponseWriter, r *http.Request, roles ...string) *User {
	u := Require(w, r)
	if u == nil {
		return nil
	}
	for _, ro := range roles {
		if u.Role == ro {
			return u
		}
	}
	http.Error(w, `{"error":"无权限执行该操作"}`, http.StatusForbidden)
	return nil
}
