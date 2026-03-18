package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samridht23/mock-api/internal/utils"
	"golang.org/x/oauth2"
)

type UserRole string

const (
	COOKIE_NAME = "_acc_tk"
)

type AuthService struct {
	DB     *pgxpool.Pool
	Google *GoogleProvider
}

type AuthStatus struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	ProfileImage string `json:"profile_image"`
}

const (
	USER_ROLE_USER      UserRole = "USER"
	USER_ROLE_ADMIN     UserRole = "ADMIN"
	USER_ROLE_MODERATOR UserRole = "MODERATOR"
)

type SessionUser struct {
	UserID    string
	IP        string
	UserAgent string
}

type User struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Picture   string    `json:"picture" db:"picture"`
	Role      UserRole  `json:"role" db:"role"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Session struct {
	ID           uuid.UUID
	UserID       string
	SessionToken string
	IpAddress    string
	UserAgent    string
	Revoked      bool
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

type SessionToken struct {
	SessionToken string    `json:"session_token"`
	SessionID    uuid.UUID `json:"session_id"`
}

func NewAuthService(db *pgxpool.Pool, google *GoogleProvider) *AuthService {
	return &AuthService{
		DB:     db,
		Google: google,
	}
}

func (a *AuthService) GenerateOAuthUrl() (string, error) {
	b := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return "", err
	}
	stateString := base64.URLEncoding.EncodeToString(b)
	oAuthStateUrl := a.Google.AuthCodeUrl(stateString, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return oAuthStateUrl, nil
}

func (a *AuthService) Upsert(ctx context.Context, user *GoogleUser) (string, error) {
	var id string
	var err error
	err = a.DB.QueryRow(ctx,
		`SELECT id from users WHERE email = $1`,
		user.Email,
	).Scan(&id)
	if err == nil {
		return id, nil
	}

	// new user
	id = user.ID
	now := time.Now().UTC()
	_, err = a.DB.Exec(ctx,
		`INSERT INTO users (id, name, email, picture, role, updated_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, user.ID, user.Name, user.Email, user.Picture, USER_ROLE_USER, now, now,
	)
	return id, err
}

func (a *AuthService) CreateSession(ctx context.Context, expires time.Time, user *SessionUser) (*Session, error) {
	var err error
	sid := uuid.New()
	sessionToken, err := generateSessionToken()
	if err != nil {
		return &Session{}, err
	}
	revoked := false
	now := time.Now().UTC()
	_, err = a.DB.Exec(ctx, `
		INSERT INTO sessions (id, user_id, session_token, ip_address, user_agent, revoked, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sid, user.UserID, sessionToken, user.IP, user.UserAgent, revoked, now, expires)
	if err != nil {
		return nil, err
	}

	s := &Session{
		ID:           sid,
		UserID:       user.UserID,
		SessionToken: sessionToken,
		IpAddress:    user.IP,
		UserAgent:    user.UserAgent,
		Revoked:      revoked,
		CreatedAt:    now,
		ExpiresAt:    expires,
	}
	return s, nil
}

func generateSessionToken() (string, error) {
	// Generate 32 random bytes (256 bits)
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	// Encode to base64 for safe transport
	return base64.URLEncoding.EncodeToString(b), nil
}

func (a *AuthService) CreateSecureToken(token interface{}) (string, error) {
	rawJSON, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session token: %w", err)
	}
	signedToken, err := utils.SignJWTToken(string(rawJSON), []byte(os.Getenv("JWT_KEY")))
	if err != nil {
		return "", fmt.Errorf("failed to sign session token: %w", err)
	}
	encryptedToken, err := utils.EncryptAESGCM(signedToken, []byte(os.Getenv("AES_BLOCK_KEY")))
	if err != nil {
		return "", fmt.Errorf("failed to encrypt session token: %w", err)
	}
	return encryptedToken, nil
}

func (a *AuthService) ParseSecureToken(token string, out interface{}) error {
	decryptedToken, err := utils.DecryptAESGCM(token, []byte(os.Getenv("AES_BLOCK_KEY")))
	if err != nil {
		return fmt.Errorf("failed to decrypt session token: %w", err)
	}
	jwtClaim, err := utils.ParseJWTToken(decryptedToken, []byte(os.Getenv("JWT_KEY")))
	if err != nil {
		return fmt.Errorf("failed to parse JWT token: %w", err)
	}
	err = json.Unmarshal([]byte(jwtClaim.Payload), out)
	if err != nil {
		return fmt.Errorf("failed to unmarshal token payload: %w", err)
	}
	return nil
}

func (a *AuthService) GetUserByID(ctx context.Context, userID string) (*User, error) {
	user := &User{}
	query := `
		SELECT id,  name, email, picture, role, updated_at, created_at
		FROM users
		WHERE id = $1
	`
	err := a.DB.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Picture,
		&user.Role,
		&user.UpdatedAt,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return user, nil
}
