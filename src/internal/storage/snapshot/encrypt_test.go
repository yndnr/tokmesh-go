package snapshot

import (
	"bytes"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     EncryptionConfig
		wantErr error
	}{
		{
			name:    "empty config is valid",
			cfg:     EncryptionConfig{},
			wantErr: nil,
		},
		{
			name:    "valid key",
			cfg:     EncryptionConfig{Key: make([]byte, 32)},
			wantErr: nil,
		},
		{
			name:    "key too short",
			cfg:     EncryptionConfig{Key: make([]byte, 8)},
			wantErr: ErrKeyTooShort,
		},
		{
			name:    "valid passphrase",
			cfg:     EncryptionConfig{Passphrase: []byte("mypassword123")},
			wantErr: nil,
		},
		{
			name:    "passphrase too weak",
			cfg:     EncryptionConfig{Passphrase: []byte("short")},
			wantErr: ErrPassphraseTooWeak,
		},
		{
			name:    "passphrase overrides key validation",
			cfg:     EncryptionConfig{Key: make([]byte, 8), Passphrase: []byte("mypassword123")},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			if err != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCipherFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     EncryptionConfig
		wantNil bool
		wantErr bool
	}{
		{
			name:    "empty config returns nil cipher",
			cfg:     EncryptionConfig{},
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "aes-gcm with key",
			cfg:     EncryptionConfig{Key: make([]byte, 32), Algorithm: "aes-gcm"},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "chacha20-poly1305 with key",
			cfg:     EncryptionConfig{Key: make([]byte, 32), Algorithm: "chacha20-poly1305"},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "default algorithm is aes-gcm",
			cfg:     EncryptionConfig{Key: make([]byte, 32)},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "passphrase derives key",
			cfg:     EncryptionConfig{Passphrase: []byte("testpassword123")},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "unsupported algorithm",
			cfg:     EncryptionConfig{Key: make([]byte, 32), Algorithm: "unknown"},
			wantNil: true,
			wantErr: true,
		},
		{
			name:    "key too short",
			cfg:     EncryptionConfig{Key: make([]byte, 8)},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cipher, _, err := NewCipherFromConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCipherFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (cipher == nil) != tt.wantNil {
				t.Errorf("NewCipherFromConfig() cipher = %v, wantNil %v", cipher, tt.wantNil)
			}
		})
	}
}

func TestDeriveKeyFromPassphrase(t *testing.T) {
	passphrase := []byte("testpassword123")

	// Test with nil salt (generates random salt).
	derived1, err := DeriveKeyFromPassphrase(passphrase, nil)
	if err != nil {
		t.Fatalf("DeriveKeyFromPassphrase() error = %v", err)
	}
	if len(derived1) != 16+32 { // salt + key
		t.Errorf("DeriveKeyFromPassphrase() length = %d, want %d", len(derived1), 16+32)
	}

	// Test with same salt produces same key.
	salt := make([]byte, 16)
	copy(salt, derived1[:16])

	derived2, err := DeriveKeyFromPassphrase(passphrase, salt)
	if err != nil {
		t.Fatalf("DeriveKeyFromPassphrase() error = %v", err)
	}
	if !bytes.Equal(derived1, derived2) {
		t.Error("DeriveKeyFromPassphrase() with same salt should produce same result")
	}

	// Test different passphrase produces different key.
	derived3, err := DeriveKeyFromPassphrase([]byte("differentpassword"), salt)
	if err != nil {
		t.Fatalf("DeriveKeyFromPassphrase() error = %v", err)
	}
	if bytes.Equal(derived1[16:], derived3[16:]) {
		t.Error("DeriveKeyFromPassphrase() with different passphrase should produce different key")
	}
}

func TestExtractKeyFromDerived(t *testing.T) {
	passphrase := []byte("testpassword123")
	derived, err := DeriveKeyFromPassphrase(passphrase, nil)
	if err != nil {
		t.Fatalf("DeriveKeyFromPassphrase() error = %v", err)
	}

	salt, key, err := ExtractKeyFromDerived(derived)
	if err != nil {
		t.Fatalf("ExtractKeyFromDerived() error = %v", err)
	}

	if len(salt) != 16 {
		t.Errorf("ExtractKeyFromDerived() salt length = %d, want 16", len(salt))
	}
	if len(key) != 32 {
		t.Errorf("ExtractKeyFromDerived() key length = %d, want 32", len(key))
	}

	// Test with too short input.
	_, _, err = ExtractKeyFromDerived(make([]byte, 10))
	if err == nil {
		t.Error("ExtractKeyFromDerived() with short input should return error")
	}
}

func TestDeriveSubkey(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	// Derive two subkeys with different info strings.
	subkey1, err := DeriveSubkey(masterKey, "snapshot", 32)
	if err != nil {
		t.Fatalf("DeriveSubkey() error = %v", err)
	}

	subkey2, err := DeriveSubkey(masterKey, "wal", 32)
	if err != nil {
		t.Fatalf("DeriveSubkey() error = %v", err)
	}

	// Subkeys should be different.
	if bytes.Equal(subkey1, subkey2) {
		t.Error("DeriveSubkey() with different info should produce different keys")
	}

	// Same info should produce same subkey.
	subkey3, err := DeriveSubkey(masterKey, "snapshot", 32)
	if err != nil {
		t.Fatalf("DeriveSubkey() error = %v", err)
	}
	if !bytes.Equal(subkey1, subkey3) {
		t.Error("DeriveSubkey() with same info should produce same key")
	}

	// Test with short master key.
	_, err = DeriveSubkey(make([]byte, 8), "test", 32)
	if err != ErrKeyTooShort {
		t.Errorf("DeriveSubkey() with short key error = %v, want %v", err, ErrKeyTooShort)
	}
}

func TestGenerateKey(t *testing.T) {
	// Test valid key generation.
	key, err := GenerateKey(32)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if len(key) != 32 {
		t.Errorf("GenerateKey() length = %d, want 32", len(key))
	}

	// Keys should be different.
	key2, err := GenerateKey(32)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if bytes.Equal(key, key2) {
		t.Error("GenerateKey() should produce different keys")
	}

	// Test with too short length.
	_, err = GenerateKey(8)
	if err != ErrKeyTooShort {
		t.Errorf("GenerateKey() with short length error = %v, want %v", err, ErrKeyTooShort)
	}
}

func TestZeroKey(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	ZeroKey(key)

	for i, b := range key {
		if b != 0 {
			t.Errorf("ZeroKey() key[%d] = %d, want 0", i, b)
		}
	}
}

func TestCipherRoundTrip(t *testing.T) {
	cfg := EncryptionConfig{
		Passphrase: []byte("testpassword123"),
		Algorithm:  "aes-gcm",
	}

	cipher, _, err := NewCipherFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewCipherFromConfig() error = %v", err)
	}

	plaintext := []byte("Hello, TokMesh snapshot encryption!")

	// Encrypt.
	ciphertext, err := cipher.Encrypt(plaintext, nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Decrypt.
	decrypted, err := cipher.Decrypt(ciphertext, nil)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}
