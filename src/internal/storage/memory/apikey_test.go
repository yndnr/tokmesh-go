package memory

import (
	"context"
	"testing"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

func TestAPIKeyStore_CRUD(t *testing.T) {
	store := NewAPIKeyStore()
	ctx := context.Background()

	// Create
	key := &domain.APIKey{
		KeyID:      "test-key-id",
		SecretHash: "test-key-hash",
		Role:       domain.RoleAdmin,
	}
	if err := store.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create conflict
	if err := store.Create(ctx, key); err != domain.ErrAPIKeyConflict {
		t.Fatalf("Create(dup) err = %v, want %v", err, domain.ErrAPIKeyConflict)
	}

	// Get
	got, err := store.Get(ctx, key.KeyID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.KeyID != key.KeyID {
		t.Fatalf("KeyID = %q, want %q", got.KeyID, key.KeyID)
	}

	// Get not found
	_, err = store.Get(ctx, "nonexistent")
	if err != domain.ErrAPIKeyNotFound {
		t.Fatalf("Get(nonexistent) err = %v, want %v", err, domain.ErrAPIKeyNotFound)
	}

	// Update
	key.Description = "updated"
	if err := store.Update(ctx, key); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = store.Get(ctx, key.KeyID)
	if got.Description != "updated" {
		t.Fatalf("Description = %q, want %q", got.Description, "updated")
	}

	// Update not found
	notExist := &domain.APIKey{KeyID: "nonexistent"}
	if err := store.Update(ctx, notExist); err != domain.ErrAPIKeyNotFound {
		t.Fatalf("Update(nonexistent) err = %v, want %v", err, domain.ErrAPIKeyNotFound)
	}

	// List
	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("len(keys) = %d, want 1", len(keys))
	}

	// Delete
	if err := store.Delete(ctx, key.KeyID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Delete not found
	if err := store.Delete(ctx, key.KeyID); err != domain.ErrAPIKeyNotFound {
		t.Fatalf("Delete(nonexistent) err = %v, want %v", err, domain.ErrAPIKeyNotFound)
	}

	// List after delete
	keys, _ = store.List(ctx)
	if len(keys) != 0 {
		t.Fatalf("len(keys) after delete = %d, want 0", len(keys))
	}
}
