package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"sync"
)

// loginRSAKey holds the RSA key pair used to encrypt passwords in transit.
// The public key is exposed to the frontend via /api/status; the frontend
// encrypts the password with RSA-OAEP before sending it, and the server
// decrypts with the private key before bcrypt comparison.
var (
	loginPrivateKey *rsa.PrivateKey
	loginPublicKey  *rsa.PublicKey
	loginPublicPEM string
	loginRSAOnce   sync.Once
	loginRSAErr    error
)

// InitLoginRSAKey initializes (or loads) the RSA key pair used for login
// password encryption. It is called once during startup. If a persisted
// PEM private key is provided (from the options table), it is loaded;
// otherwise a new 2048-bit key pair is generated.
func InitLoginRSAKey(persistedPrivateKeyPEM string) error {
	if persistedPrivateKeyPEM != "" {
		block, _ := pem.Decode([]byte(persistedPrivateKeyPEM))
		if block == nil {
			return errors.New("invalid RSA private key PEM")
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err2 != nil {
				return errors.New("failed to parse RSA private key: " + err.Error())
			}
			var ok bool
			key, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				return errors.New("parsed key is not an RSA private key")
			}
		}
		loginPrivateKey = key
		loginPublicKey = &key.PublicKey
	} else {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return errors.New("failed to generate RSA key: " + err.Error())
		}
		loginPrivateKey = key
		loginPublicKey = &key.PublicKey
	}
	// Marshal public key to PEM (SubjectPublicKeyInfo / PKIX)
	pubDER, err := x509.MarshalPKIXPublicKey(loginPublicKey)
	if err != nil {
		return errors.New("failed to marshal RSA public key: " + err.Error())
	}
	loginPublicPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}))
	return nil
}

// GetLoginRSAPublicKey returns the PEM-encoded RSA public key that the
// frontend uses to encrypt passwords before transmission.
func GetLoginRSAPublicKey() string {
	return loginPublicPEM
}

// GetLoginRSAPrivateKeyPEM returns the PKCS1 PEM encoding of the private key,
// used to persist the key across restarts (multi-instance safety).
func GetLoginRSAPrivateKeyPEM() string {
	if loginPrivateKey == nil {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(loginPrivateKey),
	}))
}

// DecryptLoginPassword decrypts a Base64-encoded RSA-OAEP ciphertext that
// the frontend produced with the login public key. Returns the plaintext
// password ready for bcrypt comparison. If the key pair is not initialized
// or decryption fails, an error is returned.
func DecryptLoginPassword(ciphertextB64 string) (string, error) {
	if loginPrivateKey == nil {
		return "", errors.New("login RSA key not initialized")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", errors.New("invalid base64 ciphertext: " + err.Error())
	}
	plaintext, err := rsa.DecryptOAEP(
		sha256.New(), // hash = SHA-256, matches frontend Web Crypto RSA-OAEP with SHA-256
		rand.Reader,
		loginPrivateKey,
		ciphertext,
		nil, // no label
	)
	if err != nil {
		return "", errors.New("RSA decryption failed: " + err.Error())
	}
	return string(plaintext), nil
}

// LoginRSAEnabled reports whether the login RSA key pair is available.
func LoginRSAEnabled() bool {
	return loginPrivateKey != nil && loginPublicPEM != ""
}

// MaybeDecryptPassword attempts to decrypt an RSA-encrypted password. If RSA
// is enabled and the value is a valid Base64 RSA-OAEP ciphertext (256 bytes →
// ~344 Base64 chars for a 2048-bit key), it decrypts and returns the plaintext.
// Otherwise it returns the original value unchanged, preserving backward
// compatibility with API clients that send plaintext passwords.
// The minCiphertextLen threshold rejects short strings early so normal
// passwords are never mistaken for ciphertext.
func MaybeDecryptPassword(password string) string {
	if !LoginRSAEnabled() || password == "" {
		return password
	}
	// A 2048-bit RSA-OAEP ciphertext is 256 bytes → 344 Base64 chars.
	// Real passwords (≤ 64 chars) cannot collide, but we still require a
	// minimum length to avoid pointless decode attempts.
	const minCiphertextLen = 256
	if len(password) < minCiphertextLen {
		return password
	}
	plaintext, err := DecryptLoginPassword(password)
	if err != nil {
		// Not a valid ciphertext — treat as plaintext for backward compatibility.
		return password
	}
	return plaintext
}
