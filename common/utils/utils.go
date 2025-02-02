package utils

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

var (
	encryptionKey []byte
	block         cipher.Block
)

func AESInit() {
	key := os.Getenv("PAYLOAD_ENCRYPTION_KEY")
	if key == "" {
		panic("PAYLOAD_ENCRYPTION_KEY environment variable is not set")
	}
	encryptionKey = []byte(key)
	var err error
	block, err = aes.NewCipher(encryptionKey)
	if err != nil {
		panic("Failed to create cipher block: " + err.Error())
	}
}

func Encrypt(data interface{}) (string, error) {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(encryptedData string) (interface{}, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	var result interface{}
	err = json.Unmarshal(ciphertext, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func RemovePrefix(key string, prefix string) string {
	return strings.TrimPrefix(key, prefix)
}

func DecryptAndConvertToJSON(encryptedData string) (interface{}, error) {
	decryptedData, err := Decrypt(encryptedData)
	if err != nil {
		return nil, err
	}

	// Ensure decryptedPayload is a string
	decryptedPayloadStr, ok := decryptedData.(string)
	if !ok {
		return nil, errors.New("decrypted payload is not a string")
	}

	// Validate the decrypted payload as JSON without unmarshaling into a map
	if !json.Valid([]byte(decryptedPayloadStr)) {
		return nil, errors.New("decrypted payload is not valid JSON")
	}

	return []byte(decryptedPayloadStr), nil
}

func ValidateAndAssignStringField(ctx context.Context, payload map[string]interface{}, fieldName string, field *string, w http.ResponseWriter) error {
	value, ok := payload[fieldName].(string)
	if !ok {
		err := errors.New("invalid or missing " + fieldName)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	*field = value
	return nil
}

func ValidateAndAssignIntField(ctx context.Context, payload map[string]interface{}, fieldName string, field *int, w http.ResponseWriter) error {
	value, ok := payload[fieldName].(int)
	if !ok {
		err := errors.New("invalid or missing " + fieldName)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	*field = value
	return nil
}
