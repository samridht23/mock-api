package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/samridht23/mock-api/internal/apperror"
	"github.com/samridht23/mock-api/internal/core"
	"github.com/samridht23/mock-api/internal/utils"
)

type CtxKey string

type AuthUser struct {
	UserID  string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Role    string `json:"role"`
}

const (
	AUTH_CONTEXT_KEY CtxKey = "auth_context"
)

func GetContext[T any](ctx context.Context, key CtxKey) (*T, bool) {
	val := ctx.Value(key)
	if user, ok := val.(*T); ok {
		return user, true
	}
	return nil, false
}

func AuthMiddleware(auth *core.AuthService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
			defer cancel()

			cookie, err := r.Cookie(core.COOKIE_NAME)
			if err != nil {
				if err == http.ErrNoCookie {
					utils.WriteError(w, apperror.ErrUnauthorized)
					return
				}
				slog.Error("error fetching request cookie", "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			var sessionToken core.SessionToken
			err = auth.ParseSecureToken(cookie.Value, &sessionToken)
			if err != nil {
				slog.Error("error parsing token", "error", err)
				utils.WriteError(w, apperror.ErrUnauthorized)
				return
			}
			s := &core.Session{
				ID: sessionToken.SessionID,
			}
			err = auth.DB.QueryRow(ctx,
				`SELECT user_id, session_token, revoked, expires_at FROM sessions WHERE id = $1`,
				s.ID).
				Scan(
					&s.UserID,
					&s.SessionToken,
					&s.Revoked,
					&s.ExpiresAt,
				)
			if err != nil || s.Revoked || time.Now().After(s.ExpiresAt) {
				utils.WriteError(w, apperror.ErrUnauthorized)
				return
			}
			user := &AuthUser{
				UserID: s.UserID,
			}
			err = auth.DB.QueryRow(ctx,
				`SELECT  name, email, picture, role FROM users WHERE id = $1`,
				s.UserID).
				Scan(
					&user.Name,
					&user.Email,
					&user.Picture,
					&user.Role,
				)
			if err != nil {
				slog.Error("error getting user auth info", "error", err)
				utils.WriteError(w, apperror.ErrUnauthorized)
				return
			}
			ctx = context.WithValue(ctx, AUTH_CONTEXT_KEY, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
