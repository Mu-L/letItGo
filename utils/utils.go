package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
)

func Encrypt(data interface{}) (string, error) {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	var encryptionKey = []byte(os.Getenv("PAYLOAD_ENCRYPTION_KEY"))
	block, err := aes.NewCipher(encryptionKey)
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

	var encryptionKey = []byte(os.Getenv("PAYLOAD_ENCRYPTION_KEY"))
	block, err := aes.NewCipher(encryptionKey)
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
