package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const BcryptCost int = 12

var (
    JWTSecret        string
    RefreshSecret    []byte
    Pepper           []byte
    JWTExpiration    time.Duration
    RefreshExpiration time.Duration
)

func InitAuthParams(jwtSecret, refreshSecret, pepper string, jwtExp, refreshExp time.Duration) {
    JWTSecret = jwtSecret
    RefreshSecret = []byte(refreshSecret)
    Pepper = []byte(pepper)
    JWTExpiration = jwtExp
    RefreshExpiration = refreshExp
}

func getEnvString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvBytes(key, fallback string) []byte {
	return []byte(getEnvString(key, fallback))
}

type RefreshToken struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type PayloadHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type Payload struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func HashPassword(password string) (string, error) {
	pepperPW := append([]byte(password), Pepper...)
	hashedPW, err := bcrypt.GenerateFromPassword(pepperPW, BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedPW), nil
}

func VerifyPassword(hashedPassword, password string) error {
	pepperPW := append([]byte(password), Pepper...)
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), pepperPW)
}

func generateSecureToken() string {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		panic("failed to generate secure random token")
	}
	return hex.EncodeToString(bytes)
}

func HashRefreshToken(token string) string {
	h := hmac.New(sha256.New, RefreshSecret)
	h.Write([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func NewRefreshToken(userID string) RefreshToken {
	return RefreshToken{
		Token:     generateSecureToken(),
		UserID:    userID,
        ExpiresAt: time.Now().Add(RefreshExpiration),
		CreatedAt: time.Now(),
	}
}

func NewPayload(userID string) Payload {
	now := time.Now()
	return Payload{
		Sub: userID,
		Iat: now.Unix(),
		Exp: now.Add(JWTExpiration).Unix(),
	}
}

func toJSON(data any) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(jsonData), nil
}

func SignPayload(secret string, payload Payload) (string, error) {
	header := PayloadHeader{Alg: "HS256", Typ: "JWT"}

	headerJSON, err := toJSON(header)
	if err != nil {
		return "", fmt.Errorf("failed to encode header: %w", err)
	}

	payloadJSON, err := toJSON(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode payload: %w", err)
	}

	unsignedToken := fmt.Sprintf("%s.%s", headerJSON, payloadJSON)

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(unsignedToken))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%s.%s.%s", headerJSON, payloadJSON, signature), nil
}

func VerifyPayload(secret, token string) (*Payload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	header, payload, signature := parts[0], parts[1], parts[2]

	headerData, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return nil, fmt.Errorf("invalid header encoding: %w", err)
	}

	var headerStruct PayloadHeader
	if err := json.Unmarshal(headerData, &headerStruct); err != nil {
		return nil, fmt.Errorf("invalid header format: %w", err)
	}

	if headerStruct.Alg != "HS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", headerStruct.Alg)
	}

	unsignedToken := fmt.Sprintf("%s.%s", header, payload)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(unsignedToken))
	expectedSig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(expectedSig), []byte(signature)) {
		return nil, fmt.Errorf("invalid signature")
	}

	jsonPayload, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding: %w", err)
	}

	var payloadData Payload
	if err := json.Unmarshal(jsonPayload, &payloadData); err != nil {
		return nil, fmt.Errorf("invalid payload format: %w", err)
	}

	if time.Now().Unix() > payloadData.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &payloadData, nil
}

func ExtractUserIDFromExpiredJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	jsonPayload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid payload encoding: %w", err)
	}

	var payloadData Payload
	if err := json.Unmarshal(jsonPayload, &payloadData); err != nil {
		return "", fmt.Errorf("invalid payload format: %w", err)
	}

	if payloadData.Sub == "" {
		return "", fmt.Errorf("no user ID in token")
	}

	return payloadData.Sub, nil
}

func ValidateTokenStructure(secret, token string) (*Payload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	header, payload, signature := parts[0], parts[1], parts[2]

	unsignedToken := fmt.Sprintf("%s.%s", header, payload)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(unsignedToken))
	expectedSig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(expectedSig), []byte(signature)) {
		return nil, fmt.Errorf("invalid signature")
	}

	jsonPayload, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding: %w", err)
	}

	var payloadData Payload
	if err := json.Unmarshal(jsonPayload, &payloadData); err != nil {
		return nil, fmt.Errorf("invalid payload format: %w", err)
	}

	return &payloadData, nil
}

