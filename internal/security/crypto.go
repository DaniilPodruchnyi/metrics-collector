package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadPublicKeyFromFile загружает RSA публичный ключ из PEM-файла.
func LoadPublicKeyFromFile(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM public key")
	}

	switch block.Type {
	case "PUBLIC KEY":
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKIX public key: %w", err)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not RSA")
		}
		return rsaPub, nil
	case "RSA PUBLIC KEY":
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 public key: %w", err)
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("unsupported public key type %q", block.Type)
	}
}

// LoadPrivateKeyFromFile загружает RSA приватный ключ из PEM-файла.
func LoadPrivateKeyFromFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM private key")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 private key: %w", err)
		}
		return priv, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		priv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
		return priv, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %q", block.Type)
	}
}

// EncryptRSA шифрует данные с использованием публичного RSA-ключа (RSA-OAEP + SHA-256).
// Подходит только для относительно небольших сообщений.
func EncryptRSA(plain []byte, pub *rsa.PublicKey) ([]byte, error) {
	if pub == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	cipher, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, plain, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa encrypt: %w", err)
	}
	return cipher, nil
}

// DecryptRSA расшифровывает данные с использованием приватного RSA-ключа (RSA-OAEP + SHA-256).
func DecryptRSA(cipher []byte, priv *rsa.PrivateKey) ([]byte, error) {
	if priv == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, cipher, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: %w", err)
	}
	return plain, nil
}

