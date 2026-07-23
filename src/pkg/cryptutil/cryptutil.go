// Package cryptutil encrypts each device's HR X-Device-Key at rest with AES-GCM,
// so a read-only browse of the database (e.g. via phpMyAdmin's erp_ro account)
// never exposes a usable HR credential. The key comes from CRED_ENC_KEY.
//
// If CRED_ENC_KEY is empty the gateway falls back to storing the secret as plain
// bytes (still only reachable by DB-authenticated users). This is documented and
// logged as a warning at startup — set CRED_ENC_KEY in any real deployment.
package cryptutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// magic prefixes the ciphertext so Decrypt can tell an encrypted value from a
// legacy/plaintext one.
var magic = []byte("zkg1")

// Cipher wraps a derived AES-256 key. A zero-value Cipher (Key == nil) means "no
// encryption configured" and passes values through unchanged.
type Cipher struct {
	aead cipher.AEAD
}

// New derives an AES-256-GCM cipher from the passphrase. An empty passphrase
// returns a pass-through cipher (Enabled() == false).
func New(passphrase string) (*Cipher, error) {
	if passphrase == "" {
		return &Cipher{}, nil
	}
	sum := sha256.Sum256([]byte(passphrase)) // 32-byte key from any-length passphrase
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Enabled reports whether encryption is active.
func (c *Cipher) Enabled() bool { return c != nil && c.aead != nil }

// Encrypt returns magic || nonce || ciphertext. With encryption disabled it
// returns the plaintext bytes unchanged.
func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	if !c.Enabled() {
		return []byte(plaintext), nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	out := make([]byte, 0, len(magic)+len(nonce)+len(ct))
	out = append(out, magic...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

// Decrypt reverses Encrypt. A value without the magic prefix is treated as
// legacy plaintext and returned as-is, so the gateway keeps working if
// CRED_ENC_KEY is introduced after some rows were stored in the clear.
func (c *Cipher) Decrypt(data []byte) (string, error) {
	if len(data) < len(magic) || string(data[:len(magic)]) != string(magic) {
		return string(data), nil // legacy / plaintext
	}
	if !c.Enabled() {
		return "", errors.New("cryptutil: value is encrypted but CRED_ENC_KEY is not set")
	}
	body := data[len(magic):]
	ns := c.aead.NonceSize()
	if len(body) < ns {
		return "", errors.New("cryptutil: ciphertext too short")
	}
	nonce, ct := body[:ns], body[ns:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
