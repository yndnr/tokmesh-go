// Package service provides domain services for TokMesh.
package service

import (
	"context"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// TestTokenService_GenerateToken tests token generation.
func TestTokenService_GenerateToken(t *testing.T) {
	svc := NewTokenService(newMockTokenRepo(), nil)

	t.Run("generate unique tokens", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			plaintext, hash, err := svc.GenerateToken()
			if err != nil {
				t.Fatalf("GenerateToken failed: %v", err)
			}

			// Check format
			if len(plaintext) < 40 {
				t.Errorf("Token too short: %d chars", len(plaintext))
			}
			if len(hash) < 60 {
				t.Errorf("Hash too short: %d chars", len(hash))
			}

			// Check uniqueness
			if tokens[plaintext] {
				t.Error("Duplicate token generated")
			}
			tokens[plaintext] = true
		}
	})

	t.Run("token format prefix", func(t *testing.T) {
		plaintext, hash, err := svc.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}

		if plaintext[:5] != "tmtk_" {
			t.Errorf("Token prefix = %s, want tmtk_", plaintext[:5])
		}
		if hash[:5] != "tmth_" {
			t.Errorf("Hash prefix = %s, want tmth_", hash[:5])
		}
	})
}

// TestTokenService_ComputeTokenHash tests token hash computation.
func TestTokenService_ComputeTokenHash(t *testing.T) {
	svc := NewTokenService(newMockTokenRepo(), nil)

	t.Run("consistent hashing", func(t *testing.T) {
		token := "tmtk_testtoken123456789012345678901234567890"
		hash1 := svc.ComputeTokenHash(token)
		hash2 := svc.ComputeTokenHash(token)

		if hash1 != hash2 {
			t.Error("Same token should produce same hash")
		}
	})

	t.Run("different tokens different hashes", func(t *testing.T) {
		token1 := "tmtk_testtoken1234567890123456789012345678901"
		token2 := "tmtk_testtoken1234567890123456789012345678902"

		hash1 := svc.ComputeTokenHash(token1)
		hash2 := svc.ComputeTokenHash(token2)

		if hash1 == hash2 {
			t.Error("Different tokens should produce different hashes")
		}
	})
}

// TestTokenService_Validate tests token validation.
func TestTokenService_Validate(t *testing.T) {
	repo := newMockTokenRepo()
	svc := NewTokenService(repo, nil)

	// Create a valid session with token
	session, _ := domain.NewSession("user123")
	plainToken, tokenHash, _ := domain.GenerateToken()
	session.TokenHash = tokenHash
	session.SetExpiration(time.Hour)
	repo.AddSession(session)

	ctx := context.Background()

	t.Run("valid token", func(t *testing.T) {
		resp, err := svc.Validate(ctx, &ValidateTokenRequest{
			Token: plainToken,
		})
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		if !resp.Valid {
			t.Error("Token should be valid")
		}
		if resp.Session == nil {
			t.Error("Session should not be nil")
		}
		if resp.Session.UserID != "user123" {
			t.Errorf("UserID = %s, want user123", resp.Session.UserID)
		}
	})

	t.Run("invalid token format", func(t *testing.T) {
		resp, err := svc.Validate(ctx, &ValidateTokenRequest{
			Token: "invalid_token",
		})
		if err == nil {
			t.Error("Expected error for invalid token format")
		}
		if resp != nil && resp.Valid {
			t.Error("Invalid token should not be valid")
		}
	})

	t.Run("non-existent token", func(t *testing.T) {
		resp, err := svc.Validate(ctx, &ValidateTokenRequest{
			Token: "tmtk_nonexistenttoken12345678901234567890123",
		})
		if err == nil {
			t.Error("Expected error for non-existent token")
		}
		if resp != nil && resp.Valid {
			t.Error("Non-existent token should not be valid")
		}
	})

	t.Run("expired session", func(t *testing.T) {
		// Create an expired session
		expiredSession, _ := domain.NewSession("expired_user")
		expiredToken, expiredHash, _ := domain.GenerateToken()
		expiredSession.TokenHash = expiredHash
		expiredSession.ExpiresAt = time.Now().Add(-time.Hour).UnixMilli() // Already expired
		repo.AddSession(expiredSession)

		resp, err := svc.Validate(ctx, &ValidateTokenRequest{
			Token: expiredToken,
		})
		if err == nil {
			t.Error("Expected error for expired session")
		}
		if resp != nil && resp.Valid {
			t.Error("Expired session token should not be valid")
		}
	})

	t.Run("validate with touch", func(t *testing.T) {
		// Get original last active from repo
		originalSession := repo.sessions[session.TokenHash]
		originalLastActive := originalSession.LastActive

		// Wait a tiny bit to ensure time difference
		time.Sleep(time.Millisecond)

		resp, err := svc.Validate(ctx, &ValidateTokenRequest{
			Token:     plainToken,
			Touch:     true,
			ClientIP:  "192.168.1.1",
			UserAgent: "TestAgent/1.0",
		})
		if err != nil {
			t.Fatalf("Validate with touch failed: %v", err)
		}
		if !resp.Valid {
			t.Error("Token should be valid")
		}

		// Check that last_active was updated in the repo
		updatedSession := repo.sessions[session.TokenHash]
		if updatedSession.LastActive <= originalLastActive {
			t.Errorf("LastActive should be updated after touch: original=%d, updated=%d",
				originalLastActive, updatedSession.LastActive)
		}
	})
}

// TestTokenService_VerifyTokenHash tests constant-time hash verification.
func TestTokenService_VerifyTokenHash(t *testing.T) {
	svc := NewTokenService(newMockTokenRepo(), nil)

	plainToken, expectedHash, _ := svc.GenerateToken()

	t.Run("correct token matches hash", func(t *testing.T) {
		if !svc.VerifyTokenHash(plainToken, expectedHash) {
			t.Error("Correct token should match its hash")
		}
	})

	t.Run("wrong token does not match hash", func(t *testing.T) {
		wrongToken := "tmtk_wrongtoken123456789012345678901234567890"
		if svc.VerifyTokenHash(wrongToken, expectedHash) {
			t.Error("Wrong token should not match hash")
		}
	})
}

// TestNonceCache tests the nonce cache for anti-replay protection.
func TestNonceCache(t *testing.T) {
	cache := NewNonceCache(100, time.Minute)

	t.Run("add and check nonce", func(t *testing.T) {
		nonce := "test-nonce-1"
		cache.Add(nonce)

		if !cache.Contains(nonce) {
			t.Error("Cache should contain added nonce")
		}
	})

	t.Run("add if absent", func(t *testing.T) {
		nonce := "test-nonce-2"

		// First add should succeed
		if !cache.AddIfAbsent(nonce) {
			t.Error("First AddIfAbsent should return true")
		}

		// Second add should fail
		if cache.AddIfAbsent(nonce) {
			t.Error("Second AddIfAbsent should return false")
		}
	})

	t.Run("capacity limit", func(t *testing.T) {
		smallCache := NewNonceCache(5, time.Minute)

		// Add more than capacity
		for i := 0; i < 10; i++ {
			smallCache.Add("nonce-" + string(rune('a'+i)))
		}

		// Size should not exceed capacity
		if smallCache.Size() > 5 {
			t.Errorf("Cache size = %d, should not exceed 5", smallCache.Size())
		}
	})

	t.Run("clear cache", func(t *testing.T) {
		cache.Add("clear-test-1")
		cache.Add("clear-test-2")
		cache.Clear()

		if cache.Size() != 0 {
			t.Errorf("Cache size after clear = %d, want 0", cache.Size())
		}
	})
}

// TestTokenService_CheckNonce tests anti-replay nonce checking.
func TestTokenService_CheckNonce(t *testing.T) {
	config := &TokenServiceConfig{
		NonceCacheSize:  1000,
		NonceTTL:        time.Minute,
		TimestampWindow: 30 * time.Second,
	}
	svc := NewTokenService(newMockTokenRepo(), config)

	ctx := context.Background()

	t.Run("valid nonce and timestamp", func(t *testing.T) {
		now := time.Now().UnixMilli()
		err := svc.CheckNonce(ctx, "unique-nonce-1", now)
		if err != nil {
			t.Errorf("CheckNonce failed: %v", err)
		}
	})

	t.Run("duplicate nonce", func(t *testing.T) {
		now := time.Now().UnixMilli()

		// First check should pass
		err := svc.CheckNonce(ctx, "duplicate-nonce", now)
		if err != nil {
			t.Fatalf("First CheckNonce failed: %v", err)
		}

		// Second check with same nonce should fail
		err = svc.CheckNonce(ctx, "duplicate-nonce", now)
		if err == nil {
			t.Error("Expected error for duplicate nonce")
		}
	})

	t.Run("timestamp out of window", func(t *testing.T) {
		// Timestamp too old (1 minute ago)
		oldTimestamp := time.Now().Add(-time.Minute).UnixMilli()
		err := svc.CheckNonce(ctx, "old-timestamp-nonce", oldTimestamp)
		if err == nil {
			t.Error("Expected error for timestamp outside window")
		}

		// Timestamp too far in future (1 minute ahead)
		futureTimestamp := time.Now().Add(time.Minute).UnixMilli()
		err = svc.CheckNonce(ctx, "future-timestamp-nonce", futureTimestamp)
		if err == nil {
			t.Error("Expected error for future timestamp outside window")
		}
	})
}
