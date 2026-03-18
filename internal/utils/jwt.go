package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaim struct {
	Payload string `json:"payload"`
	jwt.RegisteredClaims
}

func SignJWTToken(payload string, secretKey []byte) (string, error) {
	claims := JWTClaim{
		Payload: payload,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().AddDate(1, 0, 0)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w ", err)
	}
	return signedToken, nil
}

func ParseJWTToken(tokenString string, secretKey []byte) (*JWTClaim, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaim{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid jwt token")
	}
	claims, ok := token.Claims.(*JWTClaim)
	if !ok {
		return nil, fmt.Errorf("invalid jwt claim")
	}
	return claims, nil
}
