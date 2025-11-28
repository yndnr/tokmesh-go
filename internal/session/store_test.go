package session

import "testing"

func TestStore_PutGetDeleteSession(t *testing.T) {
	store := NewStore()

	sess := &Session{
		ID:       "s1",
		UserID:   "u1",
		TenantID: "t1",
		DeviceID: "d1",
		Status:   StatusActive,
	}

	store.PutSession(sess)

	got, ok := store.GetSession("s1")
	if !ok {
		t.Fatalf("expected session to exist")
	}
	if got.ID != sess.ID || got.UserID != sess.UserID {
		t.Fatalf("unexpected session: %#v", got)
	}

	store.DeleteSession("s1")
	if _, ok := store.GetSession("s1"); ok {
		t.Fatalf("expected session to be deleted")
	}
}

func TestStore_MultiDimensionalIndexes(t *testing.T) {
	store := NewStore()

	sessions := []*Session{
		{ID: "s1", UserID: "u1", TenantID: "t1", DeviceID: "d1", Status: StatusActive},
		{ID: "s2", UserID: "u1", TenantID: "t1", DeviceID: "d2", Status: StatusActive},
		{ID: "s3", UserID: "u2", TenantID: "t2", DeviceID: "d1", Status: StatusActive},
	}

	for _, s := range sessions {
		store.PutSession(s)
	}

	if got := store.SessionsByUser("u1"); len(got) != 2 {
		t.Fatalf("expected 2 sessions for user u1, got %d", len(got))
	}
	if got := store.SessionsByTenant("t1"); len(got) != 2 {
		t.Fatalf("expected 2 sessions for tenant t1, got %d", len(got))
	}
	if got := store.SessionsByDevice("d1"); len(got) != 2 {
		t.Fatalf("expected 2 sessions for device d1, got %d", len(got))
	}

	// Update one session and ensure indexes are updated
	updated := &Session{
		ID:       "s1",
		UserID:   "u1",
		TenantID: "t2",
		DeviceID: "d3",
		Status:   StatusActive,
	}
	store.PutSession(updated)

	if got := store.SessionsByTenant("t1"); len(got) != 1 {
		t.Fatalf("expected 1 session for tenant t1 after update, got %d", len(got))
	}
	if got := store.SessionsByTenant("t2"); len(got) != 2 {
		t.Fatalf("expected 2 sessions for tenant t2 after update, got %d", len(got))
	}
	if got := store.SessionsByDevice("d1"); len(got) != 1 {
		t.Fatalf("expected 1 session for device d1 after update, got %d", len(got))
	}
	if got := store.SessionsByDevice("d3"); len(got) != 1 {
		t.Fatalf("expected 1 session for device d3 after update, got %d", len(got))
	}
}

