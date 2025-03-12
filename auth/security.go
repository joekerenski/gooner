package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Password hashing using bcrypt
const BCRYPT_COST int = 8
const DefaultExpirationJWT = 1 * time.Hour

// TODO: replace with random 16-byte strings
var Pepper = []byte("lucy-is-a-good-kitty")
var Secret = "lucy-is-a-good-cat"
var RefreshSecret = []byte("lucy-is-naughty-sometimes")

func HashPassword(password string) (string, error) {

	pepperPW := append([]byte(password), Pepper...)
	hashedPW, err := bcrypt.GenerateFromPassword(pepperPW, BCRYPT_COST)
	if err != nil {
		return "", err
	}
	return string(hashedPW), nil
}

// defining JWT for auth to be stored in secure cookies
type JWT struct {
	Header    PayloadHeader `json:"header"`
	Payload   Payload       `json:"payload"`
	Signature string        `json:"signature"`
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

func NewPayload(sub string) Payload {
	return Payload{
		Sub: sub,
		Iat: time.Now().Unix(),
		Exp: time.Now().Add(DefaultExpirationJWT).Unix(),
	}
}

func ToJSON(data any) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(jsonData), nil
}

func SignPayload(secret string, payload Payload) (string, error) {
	header := PayloadHeader{Alg: "HS256", Typ: "JWT"}

	headerJSON, err := ToJSON(header)
	if err != nil {
		return "", err
	}

	payloadJSON, err := ToJSON(payload)
	if err != nil {
		return "", err
	}
	unsignedToken := fmt.Sprintf("%s.%s", headerJSON, payloadJSON)

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(unsignedToken))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	jwt := fmt.Sprintf("%s.%s.%s", headerJSON, payloadJSON, signature)
	return jwt, nil
}

func VerifyPayload(secret, token string) (*Payload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("Invalid token format!")
	}

	header := parts[0]
	payload := parts[1]
	signature := parts[2]

	unsignedToken := fmt.Sprintf("%s.%s", header, payload)

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(unsignedToken))
	expectedSig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	if expectedSig != signature {
		return nil, fmt.Errorf("Invalid signature!")
	}

	jsonPayload, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}

	var payloadData Payload
	if err := json.Unmarshal(jsonPayload, &payloadData); err != nil {
		return nil, err
	}

	if time.Now().Unix() > payloadData.Exp {
		return nil, fmt.Errorf("Token expired!")
	}
	return &payloadData, nil
}
