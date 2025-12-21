// Package adaptive provides adaptive encryption with automatic algorithm selection.
package adaptive

import (
	"bytes"
	"testing"
)

// Test key sizes
var (
	key16 = make([]byte, 16) // AES-128
	key24 = make([]byte, 24) // AES-192
	key32 = make([]byte, 32) // AES-256
)

func init() {
	// Initialize test keys with deterministic values
	for i := range key16 {
		key16[i] = byte(i)
	}
	for i := range key24 {
		key24[i] = byte(i)
	}
	for i := range key32 {
		key32[i] = byte(i)
	}
}

func TestNew(t *testing.T) {
	cipher, err := New(key32)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if cipher == nil {
		t.Fatal("New() returned nil cipher")
	}

	// Should return AES-GCM on amd64/arm64, ChaCha20 otherwise
	cipherType := cipher.Type()
	if cipherType != CipherAESGCM && cipherType != CipherChaCha20 {
		t.Errorf("New() returned unknown cipher type: %s", cipherType)
	}
}

func TestNewWithType_AESGCM(t *testing.T) {
	cipher, err := NewWithType(key32, CipherAESGCM)
	if err != nil {
		t.Fatalf("NewWithType(AES-GCM) error = %v", err)
	}

	if cipher.Type() != CipherAESGCM {
		t.Errorf("NewWithType(AES-GCM) type = %s, want %s", cipher.Type(), CipherAESGCM)
	}
}

func TestNewWithType_ChaCha20(t *testing.T) {
	cipher, err := NewWithType(key32, CipherChaCha20)
	if err != nil {
		t.Fatalf("NewWithType(ChaCha20) error = %v", err)
	}

	if cipher.Type() != CipherChaCha20 {
		t.Errorf("NewWithType(ChaCha20) type = %s, want %s", cipher.Type(), CipherChaCha20)
	}
}

func TestNewWithType_Unknown(t *testing.T) {
	_, err := NewWithType(key32, "unknown-cipher")
	if err == nil {
		t.Error("NewWithType(unknown) should return error")
	}
}

func TestNewAESGCM(t *testing.T) {
	tests := []struct {
		name    string
		key     []byte
		wantErr bool
	}{
		{"AES-128", key16, false},
		{"AES-192", key24, false},
		{"AES-256", key32, false},
		{"Invalid 15 bytes", make([]byte, 15), true},
		{"Invalid 17 bytes", make([]byte, 17), true},
		{"Invalid 31 bytes", make([]byte, 31), true},
		{"Invalid 33 bytes", make([]byte, 33), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cipher, err := NewAESGCM(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("NewAESGCM() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("NewAESGCM() error = %v", err)
				}
				if cipher == nil {
					t.Error("NewAESGCM() returned nil cipher")
				}
			}
		})
	}
}

func TestNewChaCha20(t *testing.T) {
	tests := []struct {
		name    string
		key     []byte
		wantErr bool
	}{
		{"Valid 32 bytes", key32, false},
		{"Invalid 16 bytes", key16, true},
		{"Invalid 24 bytes", key24, true},
		{"Invalid 31 bytes", make([]byte, 31), true},
		{"Invalid 33 bytes", make([]byte, 33), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cipher, err := NewChaCha20(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("NewChaCha20() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("NewChaCha20() error = %v", err)
				}
				if cipher == nil {
					t.Error("NewChaCha20() returned nil cipher")
				}
			}
		})
	}
}

func TestAESGCM_EncryptDecrypt(t *testing.T) {
	cipher, err := NewAESGCM(key32)
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	testEncryptDecrypt(t, cipher)
}

func TestChaCha20_EncryptDecrypt(t *testing.T) {
	cipher, err := NewChaCha20(key32)
	if err != nil {
		t.Fatalf("NewChaCha20() error = %v", err)
	}

	testEncryptDecrypt(t, cipher)
}

func testEncryptDecrypt(t *testing.T, cipher Cipher) {
	tests := []struct {
		name           string
		plaintext      []byte
		additionalData []byte
	}{
		{"Empty", []byte{}, nil},
		{"Simple", []byte("hello world"), nil},
		{"With AAD", []byte("secret data"), []byte("authenticated")},
		{"Large", bytes.Repeat([]byte("A"), 1024), nil},
		{"Binary", []byte{0x00, 0xFF, 0x7F, 0x80}, []byte{0x01, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := cipher.Encrypt(tt.plaintext, tt.additionalData)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Ciphertext should be longer than plaintext (nonce + tag)
			expectedMinLen := len(tt.plaintext) + cipher.NonceSize() + cipher.Overhead()
			if len(ciphertext) < expectedMinLen {
				t.Errorf("Encrypt() ciphertext length = %d, want >= %d", len(ciphertext), expectedMinLen)
			}

			plaintext, err := cipher.Decrypt(ciphertext, tt.additionalData)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if !bytes.Equal(plaintext, tt.plaintext) {
				t.Errorf("Decrypt() plaintext = %v, want %v", plaintext, tt.plaintext)
			}
		})
	}
}

func TestAESGCM_DecryptTampered(t *testing.T) {
	cipher, err := NewAESGCM(key32)
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	testDecryptTampered(t, cipher)
}

func TestChaCha20_DecryptTampered(t *testing.T) {
	cipher, err := NewChaCha20(key32)
	if err != nil {
		t.Fatalf("NewChaCha20() error = %v", err)
	}

	testDecryptTampered(t, cipher)
}

func testDecryptTampered(t *testing.T, cipher Cipher) {
	plaintext := []byte("secret message")
	aad := []byte("authenticated data")

	ciphertext, err := cipher.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Tamper with ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = cipher.Decrypt(tampered, aad)
	if err == nil {
		t.Error("Decrypt() should fail for tampered ciphertext")
	}

	// Wrong AAD
	_, err = cipher.Decrypt(ciphertext, []byte("wrong aad"))
	if err == nil {
		t.Error("Decrypt() should fail for wrong AAD")
	}
}

func TestAESGCM_DecryptTooShort(t *testing.T) {
	cipher, err := NewAESGCM(key32)
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	testDecryptTooShort(t, cipher)
}

func TestChaCha20_DecryptTooShort(t *testing.T) {
	cipher, err := NewChaCha20(key32)
	if err != nil {
		t.Fatalf("NewChaCha20() error = %v", err)
	}

	testDecryptTooShort(t, cipher)
}

func testDecryptTooShort(t *testing.T, cipher Cipher) {
	// Ciphertext shorter than nonce
	short := make([]byte, cipher.NonceSize()-1)
	_, err := cipher.Decrypt(short, nil)
	if err == nil {
		t.Error("Decrypt() should fail for ciphertext shorter than nonce")
	}
}

func TestAESGCM_NonceSize(t *testing.T) {
	cipher, err := NewAESGCM(key32)
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	// AES-GCM standard nonce size is 12 bytes
	if cipher.NonceSize() != 12 {
		t.Errorf("NonceSize() = %d, want 12", cipher.NonceSize())
	}
}

func TestChaCha20_NonceSize(t *testing.T) {
	cipher, err := NewChaCha20(key32)
	if err != nil {
		t.Fatalf("NewChaCha20() error = %v", err)
	}

	// ChaCha20-Poly1305 nonce size is 12 bytes
	if cipher.NonceSize() != 12 {
		t.Errorf("NonceSize() = %d, want 12", cipher.NonceSize())
	}
}

func TestAESGCM_Overhead(t *testing.T) {
	cipher, err := NewAESGCM(key32)
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	// AES-GCM tag size is 16 bytes
	if cipher.Overhead() != 16 {
		t.Errorf("Overhead() = %d, want 16", cipher.Overhead())
	}
}

func TestChaCha20_Overhead(t *testing.T) {
	cipher, err := NewChaCha20(key32)
	if err != nil {
		t.Fatalf("NewChaCha20() error = %v", err)
	}

	// ChaCha20-Poly1305 tag size is 16 bytes
	if cipher.Overhead() != 16 {
		t.Errorf("Overhead() = %d, want 16", cipher.Overhead())
	}
}

func TestAESGCM_Type(t *testing.T) {
	cipher, err := NewAESGCM(key32)
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	if cipher.Type() != CipherAESGCM {
		t.Errorf("Type() = %s, want %s", cipher.Type(), CipherAESGCM)
	}
}

func TestChaCha20_Type(t *testing.T) {
	cipher, err := NewChaCha20(key32)
	if err != nil {
		t.Fatalf("NewChaCha20() error = %v", err)
	}

	if cipher.Type() != CipherChaCha20 {
		t.Errorf("Type() = %s, want %s", cipher.Type(), CipherChaCha20)
	}
}

func TestEncrypt_Uniqueness(t *testing.T) {
	cipher, err := NewAESGCM(key32)
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	plaintext := []byte("same plaintext")
	results := make(map[string]bool)

	// Same plaintext should produce different ciphertexts (random nonce)
	for i := 0; i < 10; i++ {
		ciphertext, err := cipher.Encrypt(plaintext, nil)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}
		key := string(ciphertext)
		if results[key] {
			t.Error("Encrypt() produced duplicate ciphertext (nonce collision)")
		}
		results[key] = true
	}
}

// Benchmark tests
func BenchmarkAESGCM_Encrypt_1KB(b *testing.B) {
	cipher, _ := NewAESGCM(key32)
	plaintext := bytes.Repeat([]byte("A"), 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Encrypt(plaintext, nil)
	}
}

func BenchmarkChaCha20_Encrypt_1KB(b *testing.B) {
	cipher, _ := NewChaCha20(key32)
	plaintext := bytes.Repeat([]byte("A"), 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Encrypt(plaintext, nil)
	}
}

func BenchmarkAESGCM_Decrypt_1KB(b *testing.B) {
	cipher, _ := NewAESGCM(key32)
	plaintext := bytes.Repeat([]byte("A"), 1024)
	ciphertext, _ := cipher.Encrypt(plaintext, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Decrypt(ciphertext, nil)
	}
}

func BenchmarkChaCha20_Decrypt_1KB(b *testing.B) {
	cipher, _ := NewChaCha20(key32)
	plaintext := bytes.Repeat([]byte("A"), 1024)
	ciphertext, _ := cipher.Encrypt(plaintext, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Decrypt(ciphertext, nil)
	}
}
