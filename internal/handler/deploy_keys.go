package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

const encryptionKey = "labuh-deploy-key-encryption-key-32b!!"

type DeployKeysHandler struct {
	keyRepo *repo.DeployKeyRepo
	projRepo *repo.ProjectRepo
}

func NewDeployKeysHandler(keyRepo *repo.DeployKeyRepo, projRepo *repo.ProjectRepo) *DeployKeysHandler {
	return &DeployKeysHandler{
		keyRepo: keyRepo,
		projRepo: projRepo,
	}
}

func (h *DeployKeysHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	keys, err := h.keyRepo.GetByProjectID(projectID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.DeployKeysPage(projectID, keys))).ServeHTTP(w, r)
}

func (h *DeployKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = "Deploy Key"
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	publicKey := "ssh-ed25519 " + base64.StdEncoding.EncodeToString(pub)
	privateKey := base64.StdEncoding.EncodeToString(priv)

	fingerprintHash := sha256.Sum256(pub)
	fingerprintStr := base64.StdEncoding.EncodeToString(fingerprintHash[:])

	encryptedPrivateKey, err := encryptAES(privateKey, encryptionKey)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_, err = h.keyRepo.Create(models.CreateDeployKeyInput{
		ProjectID:           projectID,
		Name:                name,
		PublicKey:           publicKey,
		PrivateKeyEncrypted: encryptedPrivateKey,
		Fingerprint:         fingerprintStr,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects/"+projectID+"/deploy-keys")
	w.WriteHeader(http.StatusOK)
}

func (h *DeployKeysHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.keyRepo.Delete(id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects/"+chi.URLParam(r, "project_id")+"/deploy-keys")
	w.WriteHeader(http.StatusOK)
}

func encryptAES(plaintext, keyString string) (string, error) {
	key := []byte(keyString)
	if len(key) > 32 {
		key = key[:32]
	}
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	plaintextBytes := []byte(plaintext)
	ciphertext := make([]byte, len(plaintextBytes))
	stream.XORKeyStream(ciphertext, plaintextBytes)

	result := append(iv, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}

func decryptAES(ciphertextB64, keyString string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}

	key := []byte(keyString)
	if len(key) > 32 {
		key = key[:32]
	}
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}
