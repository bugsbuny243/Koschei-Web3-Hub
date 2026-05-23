package services

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	b, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), e
}
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func CreateJWT(secret, userID, email, tier string) (string, error) {
	claims := jwt.MapClaims{"sub": userID, "email": email, "tier": tier, "exp": time.Now().Add(7 * 24 * time.Hour).Unix()}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}
