package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/samridht23/mock-api/internal/apperror"
	"github.com/samridht23/mock-api/internal/core"
	"github.com/samridht23/mock-api/internal/middleware"
	"github.com/samridht23/mock-api/internal/utils"
)

// get /auth/google
func GoogleLogin(auth *core.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url, err := auth.GenerateOAuthUrl()
		if err != nil {
			slog.Error("error generating login oauth url", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

func GoogleCallback(auth *core.AuthService, googleHttp *core.GoogleHTTPService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		code := r.URL.Query().Get("code")
		if code == "" {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "Callback code missing")
			return
		}
		token, err := auth.Google.Exchange(ctx, code)
		if err != nil {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		user, err := googleHttp.GetUserInfo(ctx, token.AccessToken)
		if err != nil {
			slog.Error("google user info http request failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		userID, err := auth.Upsert(ctx, user)
		if err != nil {
			slog.Error("auth upsert error", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		userIP := utils.GetIP(r)
		userAgent := utils.GetUserAgent(r)
		u := &core.SessionUser{
			UserID:    userID,
			IP:        userIP,
			UserAgent: userAgent,
		}

		expires := time.Now().UTC().Add(30 * 24 * time.Hour)

		session, err := auth.CreateSession(ctx, expires, u)
		if err != nil {
			slog.Error("error creating user session", "user_id", userID, "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		payload := &core.SessionToken{
			SessionToken: session.SessionToken,
			SessionID:    session.ID,
		}

		cookieToken, err := auth.CreateSecureToken(payload)
		if err != nil {
			slog.Error("error creating secure token", "user_id", userID, "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     core.COOKIE_NAME,
			Value:    cookieToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			Expires:  expires,
		})

		http.Redirect(w, r, os.Getenv("CLIENT_URL"), http.StatusTemporaryRedirect)
		return
	}
}

func AuthStatus(auth *core.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authContext, ok := r.Context().Value(middleware.AUTH_CONTEXT_KEY).(*middleware.AuthUser)
		if !ok {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		user, err := auth.GetUserByID(ctx, authContext.UserID)
		if err != nil {
			slog.Error("error getting user info", "user_id", authContext.UserID, "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		authStatus := &core.AuthStatus{
			Name:         user.Name,
			Role:         string(user.Role),
			ProfileImage: user.Picture,
		}
		utils.WriteSuccess(w, http.StatusOK, authStatus)
	}
}

func Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		http.SetCookie(w, &http.Cookie{
			Name:     core.COOKIE_NAME,
			Value:    "",
			Path:     "/",
			MaxAge:   -1, // delete immediately
			HttpOnly: true,
			Secure:   false,                // keep TRUE if you used it when setting
			SameSite: http.SameSiteLaxMode, // must match original
		})

		utils.WriteSuccess(w, http.StatusOK, map[string]string{
			"message": "Logged out successfully",
		})

	}
}
