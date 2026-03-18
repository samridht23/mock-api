package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	GOOGLE_USER_INFO_ENDPOINT    = "https://www.googleapis.com/oauth2/v2/userinfo"
	GOOGLE_USER_EMAIL_ENDPOINT   = "https://www.googleapis.com/auth/userinfo.email"
	GOOGLE_USER_PROFILE_ENDPOINT = "https://www.googleapis.com/auth/userinfo.profile"
	GOOGLE_TOKEN_INFO_ENDPOINT   = "https://oauth2.googleapis.com/tokeninfo"

	CLIENT_DEFAULT_TIMEOUT = 10 * time.Second
	CLIENT_MAX_BODY_SIZE   = 1 << 20 //1MB
)

type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

type GoogleAPIError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

type GoogleToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type GoogleHTTPService struct {
	client HTTPClient
}

func NewGoogleHTTPService(client HTTPClient) *GoogleHTTPService {
	if client == nil {
		client = &http.Client{
			Timeout: CLIENT_DEFAULT_TIMEOUT,
		}
	}
	return &GoogleHTTPService{
		client: client,
	}
}

func (s *GoogleHTTPService) GetUserInfo(ctx context.Context, accessToken string) (*GoogleUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GOOGLE_USER_INFO_ENDPOINT, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user google info request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s.handleErrorResponse(resp)
	}
	userData, err := s.parseResponse(resp)
	if err != nil {
		return nil, err
	}
	return userData, nil
}

func (s *GoogleHTTPService) handleErrorResponse(resp *http.Response) (*GoogleUser, error) {
	var apiErr GoogleAPIError
	limitedReader := io.LimitReader(resp.Body, CLIENT_MAX_BODY_SIZE)
	err := json.NewDecoder(limitedReader).Decode(&apiErr)
	if err != nil {
		return nil, fmt.Errorf("unexpected status %d: failed to decode error response", resp.StatusCode)
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed: %s (code %d)", apiErr.Error.Message, apiErr.Error.Code)
	case http.StatusForbidden:
		return nil, fmt.Errorf("insufficient permissions: %s", apiErr.Error.Message)
	default:
		return nil, fmt.Errorf("api request failed: %s (status %s)", apiErr.Error.Message, apiErr.Error.Status)
	}
}

func (s *GoogleHTTPService) parseResponse(resp *http.Response) (*GoogleUser, error) {
	limitedReader := io.LimitReader(resp.Body, CLIENT_MAX_BODY_SIZE)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var userData GoogleUser
	if err := json.Unmarshal(body, &userData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w (body: %s)", err, string(body))
	}
	if userData.ID == "" {
		return nil, fmt.Errorf("invalid user data: missing ID")
	}
	return &userData, nil
}
