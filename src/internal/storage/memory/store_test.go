package memory

import (
	"context"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

func TestStore_CreateIndexesAndLookup(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_test_1"
	s.SetExpiration(time.Hour)

	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != s.ID {
		t.Fatalf("Get ID = %q, want %q", got.ID, s.ID)
	}

	byToken, err := store.GetByToken(ctx, s.TokenHash)
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if byToken.ID != s.ID {
		t.Fatalf("GetByToken ID = %q, want %q", byToken.ID, s.ID)
	}

	list, err := store.ListByUserID(ctx, "u1")
	if err != nil {
		t.Fatalf("ListByUserID: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}

	count, err := store.CountByUserID(ctx, "u1")
	if err != nil {
		t.Fatalf("CountByUserID: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountByUserID = %d, want 1", count)
	}
}

func TestStore_Quota(t *testing.T) {
	store := New(WithMaxSessionsPerUser(1))
	ctx := context.Background()

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_q_1"
	s1.SetExpiration(time.Hour)
	if err := store.Create(ctx, s1); err != nil {
		t.Fatalf("Create 1: %v", err)
	}

	s2, _ := domain.NewSession("u1")
	s2.TokenHash = "tmth_q_2"
	s2.SetExpiration(time.Hour)
	if err := store.Create(ctx, s2); err != domain.ErrSessionQuotaExceeded {
		t.Fatalf("Create 2 err = %v, want %v", err, domain.ErrSessionQuotaExceeded)
	}
}

func TestStore_UpdateVersionConflict(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_v_1"
	s.SetExpiration(time.Hour)
	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	s.Data["k"] = "v"
	if err := store.Update(ctx, s, 999); err != domain.ErrSessionVersionConflict {
		t.Fatalf("Update err = %v, want %v", err, domain.ErrSessionVersionConflict)
	}
}

func TestStore_DeleteCleansIndexes(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_d_1"
	s.SetExpiration(time.Hour)
	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Delete(ctx, s.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := store.Get(ctx, s.ID); err != domain.ErrSessionNotFound {
		t.Fatalf("Get err = %v, want %v", err, domain.ErrSessionNotFound)
	}
	if _, err := store.GetByToken(ctx, s.TokenHash); err != domain.ErrTokenInvalid {
		t.Fatalf("GetByToken err = %v, want %v", err, domain.ErrTokenInvalid)
	}
}

func TestStore_UpdateChangesTokenIndex(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_old"
	s.SetExpiration(time.Hour)
	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Change token hash and update with correct expected version.
	expectedVersion := s.Version
	s.TokenHash = "tmth_new"
	if err := store.Update(ctx, s, expectedVersion); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := store.GetByToken(ctx, "tmth_old"); err != domain.ErrTokenInvalid {
		t.Fatalf("GetByToken(old) err = %v, want %v", err, domain.ErrTokenInvalid)
	}
	got, err := store.GetByToken(ctx, "tmth_new")
	if err != nil {
		t.Fatalf("GetByToken(new): %v", err)
	}
	if got.ID != s.ID {
		t.Fatalf("GetByToken(new) ID = %q, want %q", got.ID, s.ID)
	}
}

func TestStore_DeleteByToken(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_del_by_token"
	s.SetExpiration(time.Hour)
	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.DeleteByToken(ctx, s.TokenHash); err != nil {
		t.Fatalf("DeleteByToken: %v", err)
	}
	if _, err := store.Get(ctx, s.ID); err != domain.ErrSessionNotFound {
		t.Fatalf("Get err = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestStore_DeleteByUserID(t *testing.T) {
	store := New()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s, _ := domain.NewSession("u1")
		s.TokenHash = "tmth_bulk_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		if err := store.Create(ctx, s); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	deleted, err := store.DeleteByUserID(ctx, "u1")
	if err != nil {
		t.Fatalf("DeleteByUserID: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}
}

func TestStore_LoadFromSnapshotAndCleanupExpired(t *testing.T) {
	store := New()

	expired, _ := domain.NewSession("u1")
	expired.TokenHash = "tmth_expired"
	expired.ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()

	active, _ := domain.NewSession("u1")
	active.TokenHash = "tmth_active"
	active.SetExpiration(time.Hour)

	if err := store.LoadFromSnapshot([]*domain.Session{expired, active}); err != nil {
		t.Fatalf("LoadFromSnapshot: %v", err)
	}

	removed := store.CleanupExpired()
	if removed != 1 {
		t.Fatalf("CleanupExpired removed = %d, want 1", removed)
	}
	if store.Count() != 1 {
		t.Fatalf("Count = %d, want 1", store.Count())
	}
}

func TestStore_ListWithUserFilterAndPaging(t *testing.T) {
	store := New()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s, _ := domain.NewSession("u1")
		s.TokenHash = "tmth_list_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		if err := store.Create(ctx, s); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	filter := &service.SessionFilter{
		UserID:    "u1",
		Page:      1,
		PageSize:  2,
		SortBy:    "created_at",
		SortOrder: "desc",
	}
	sessions, total, err := store.List(ctx, filter)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
}

func TestStore_Touch(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_touch"
	s.SetExpiration(time.Hour)
	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Touch(ctx, s.ID, "1.2.3.4", "ua"); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	got, err := store.Get(ctx, s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastAccessIP != "1.2.3.4" || got.LastAccessUA != "ua" {
		t.Fatalf("touch fields not updated: %+v", got)
	}
}

func TestStore_TouchExpired(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_touch_exp"
	s.ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()
	store.sessions.Set(s.ID, s)
	store.tokens.Set(s.TokenHash, s.ID)

	if err := store.Touch(ctx, s.ID, "1.2.3.4", "ua"); err != domain.ErrSessionExpired {
		t.Fatalf("Touch err = %v, want %v", err, domain.ErrSessionExpired)
	}
}

func TestStore_TouchNotFound(t *testing.T) {
	store := New()
	ctx := context.Background()

	if err := store.Touch(ctx, "nonexistent", "1.2.3.4", "ua"); err != domain.ErrSessionNotFound {
		t.Fatalf("Touch err = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestStore_CountByUser(t *testing.T) {
	store := New()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s, _ := domain.NewSession("user_count")
		s.TokenHash = "tmth_cbu_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		if err := store.Create(ctx, s); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	count := store.CountByUser("user_count")
	if count != 3 {
		t.Fatalf("CountByUser = %d, want 3", count)
	}

	count = store.CountByUser("nonexistent")
	if count != 0 {
		t.Fatalf("CountByUser(nonexistent) = %d, want 0", count)
	}
}

func TestStore_Scan(t *testing.T) {
	store := New()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s, _ := domain.NewSession("scan_user")
		s.TokenHash = "tmth_scan_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		if err := store.Create(ctx, s); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	// Count all sessions
	count := 0
	store.Scan(func(s *domain.Session) bool {
		count++
		return true
	})
	if count != 5 {
		t.Fatalf("Scan counted = %d, want 5", count)
	}

	// Stop early
	count = 0
	store.Scan(func(s *domain.Session) bool {
		count++
		return count < 3
	})
	if count != 3 {
		t.Fatalf("Scan early stop = %d, want 3", count)
	}
}

func TestStore_All(t *testing.T) {
	store := New()
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		s, _ := domain.NewSession("all_user")
		s.TokenHash = "tmth_all_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		if err := store.Create(ctx, s); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	all := store.All()
	if len(all) != 4 {
		t.Fatalf("len(All()) = %d, want 4", len(all))
	}

	// Verify that returned sessions are clones
	for _, s := range all {
		if s.UserID != "all_user" {
			t.Fatalf("session.UserID = %q, want %q", s.UserID, "all_user")
		}
	}
}

func TestStore_GetSessionByTokenHash(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_by_hash"
	s.SetExpiration(time.Hour)
	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.GetSessionByTokenHash(ctx, s.TokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash: %v", err)
	}
	if got.ID != s.ID {
		t.Fatalf("ID = %q, want %q", got.ID, s.ID)
	}

	_, err = store.GetSessionByTokenHash(ctx, "nonexistent")
	if err != domain.ErrTokenInvalid {
		t.Fatalf("GetSessionByTokenHash(nonexistent) err = %v, want %v", err, domain.ErrTokenInvalid)
	}
}

func TestStore_UpdateSession(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_update_sess"
	s.SetExpiration(time.Hour)
	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update session data
	s.Data["key"] = "value"
	s.LastAccessIP = "9.9.9.9"
	if err := store.UpdateSession(ctx, s); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	got, err := store.Get(ctx, s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Data["key"] != "value" {
		t.Fatalf("Data[key] = %q, want %q", got.Data["key"], "value")
	}
	if got.LastAccessIP != "9.9.9.9" {
		t.Fatalf("LastAccessIP = %q, want %q", got.LastAccessIP, "9.9.9.9")
	}
}

func TestStore_UpdateSessionNotFound(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_update_notfound"
	s.SetExpiration(time.Hour)

	if err := store.UpdateSession(ctx, s); err != domain.ErrSessionNotFound {
		t.Fatalf("UpdateSession err = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestStore_GetExpired(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_get_exp"
	s.ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()
	store.sessions.Set(s.ID, s)
	store.tokens.Set(s.TokenHash, s.ID)

	_, err := store.Get(ctx, s.ID)
	if err != domain.ErrSessionExpired {
		t.Fatalf("Get err = %v, want %v", err, domain.ErrSessionExpired)
	}
}

func TestStore_GetByTokenExpired(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_bytoken_exp"
	s.ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()
	store.sessions.Set(s.ID, s)
	store.tokens.Set(s.TokenHash, s.ID)
	store.userIndex.Add(s.UserID, s.ID)

	_, err := store.GetByToken(ctx, s.TokenHash)
	if err != domain.ErrSessionExpired {
		t.Fatalf("GetByToken err = %v, want %v", err, domain.ErrSessionExpired)
	}
}

func TestStore_CreateConflict(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_conflict"
	s.SetExpiration(time.Hour)

	if err := store.Create(ctx, s); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	if err := store.Create(ctx, s); err != domain.ErrSessionConflict {
		t.Fatalf("Create 2 err = %v, want %v", err, domain.ErrSessionConflict)
	}
}

func TestStore_CreateTokenConflict(t *testing.T) {
	store := New()
	ctx := context.Background()

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_token_conflict"
	s1.SetExpiration(time.Hour)
	if err := store.Create(ctx, s1); err != nil {
		t.Fatalf("Create 1: %v", err)
	}

	s2, _ := domain.NewSession("u2")
	s2.TokenHash = "tmth_token_conflict" // Same token hash
	s2.SetExpiration(time.Hour)
	if err := store.Create(ctx, s2); err != domain.ErrTokenHashConflict {
		t.Fatalf("Create 2 err = %v, want %v", err, domain.ErrTokenHashConflict)
	}
}

func TestStore_UpdateNotFound(t *testing.T) {
	store := New()
	ctx := context.Background()

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_update_nf"
	s.SetExpiration(time.Hour)

	if err := store.Update(ctx, s, 0); err != domain.ErrSessionNotFound {
		t.Fatalf("Update err = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestStore_DeleteNotFound(t *testing.T) {
	store := New()
	ctx := context.Background()

	if err := store.Delete(ctx, "nonexistent"); err != domain.ErrSessionNotFound {
		t.Fatalf("Delete err = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestStore_DeleteByTokenNotFound(t *testing.T) {
	store := New()
	ctx := context.Background()

	if err := store.DeleteByToken(ctx, "nonexistent"); err != domain.ErrTokenInvalid {
		t.Fatalf("DeleteByToken err = %v, want %v", err, domain.ErrTokenInvalid)
	}
}

func TestStore_ListWithActiveFilter(t *testing.T) {
	store := New()
	ctx := context.Background()

	// Create active session
	active, _ := domain.NewSession("filter_user")
	active.TokenHash = "tmth_filter_active"
	active.SetExpiration(time.Hour)
	if err := store.Create(ctx, active); err != nil {
		t.Fatalf("Create active: %v", err)
	}

	// Create expired session (manually set)
	expired, _ := domain.NewSession("filter_user")
	expired.TokenHash = "tmth_filter_expired"
	expired.ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()
	store.sessions.Set(expired.ID, expired)
	store.tokens.Set(expired.TokenHash, expired.ID)
	store.userIndex.Add(expired.UserID, expired.ID)

	// Filter for active only using Status
	filter := &service.SessionFilter{
		Status:   "active",
		Page:     1,
		PageSize: 10,
	}
	sessions, total, err := store.List(ctx, filter)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}
}

func TestStore_ListSortByLastActive(t *testing.T) {
	store := New()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s, _ := domain.NewSession("sort_user")
		s.TokenHash = "tmth_sort_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		s.LastActive = time.Now().Add(time.Duration(i) * time.Minute).UnixMilli()
		if err := store.Create(ctx, s); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	filter := &service.SessionFilter{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_active",
		SortOrder: "asc",
	}
	sessions, _, err := store.List(ctx, filter)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("len(sessions) = %d, want 3", len(sessions))
	}
	// Verify ascending order
	for i := 1; i < len(sessions); i++ {
		if sessions[i].LastActive < sessions[i-1].LastActive {
			t.Fatalf("sessions not sorted by last_active asc")
		}
	}
}
