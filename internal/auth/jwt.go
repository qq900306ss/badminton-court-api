package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	OrgID    string `json:"org_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	PlayerID string `json:"player_id,omitempty"`
	Name     string `json:"name,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(orgID, email, role string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	claims := Claims{
		OrgID: orgID,
		Email: email,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GeneratePlayerToken issues a drop-in player JWT (role "player" + player_id).
// Longer-lived than leader tokens so players stay logged in across sessions.
func GeneratePlayerToken(playerID, name string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	claims := Claims{
		PlayerID: playerID,
		Name:     name,
		Role:     "player",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(90 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenStr string) (*Claims, error) {
	secret := os.Getenv("JWT_SECRET")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
