package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
)

var masterKey []byte
var initialized bool

func Init(masterKeyEnv string) error {
	key := os.Getenv(masterKeyEnv)
	if key == "" {
		return errors.New("master key not set in environment variable " + masterKeyEnv)
	}

	hash := sha256.Sum256([]byte(key))
	masterKey = hash[:]
	initialized = true
	return nil
}

func IsInitialized() bool {
	return initialized
}

func GetMasterKey() ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, errors.New("encryption not initialized")
	}
	return masterKey, nil
}

func Encrypt(plaintext string) (string, error) {
	key, err := GetMasterKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(ciphertext string) (string, error) {
	key, err := GetMasterKey()
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, "ENC:")
}

func EncryptedValue(encryptedBase64 string) string {
	return "ENC:" + encryptedBase64
}
