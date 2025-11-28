package session

import "sync"

// Store 维护 Session 的内存状态与多维索引。
// P1 阶段仅提供单节点内存实现，不负责持久化。
type Store struct {
	mu sync.RWMutex

	sessionsByID      map[string]*Session
	sessionsByUser    map[string]map[string]struct{}   // userID -> set(sessionID)
	sessionsByTenant  map[string]map[string]struct{}   // tenantID -> set(sessionID)
	sessionsByDevice  map[string]map[string]struct{}   // deviceID -> set(sessionID)
}

func NewStore() *Store {
	return &Store{
		sessionsByID:     make(map[string]*Session),
		sessionsByUser:   make(map[string]map[string]struct{}),
		sessionsByTenant: make(map[string]map[string]struct{}),
		sessionsByDevice: make(map[string]map[string]struct{}),
	}
}

// GetSession 按 SessionID 查询。
func (s *Store) GetSession(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessionsByID[id]
	return session, ok
}

// PutSession 插入或更新 Session，并维护多维索引。
// 调用方应保证 ID 非空且唯一。
func (s *Store) PutSession(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已存在，先从旧索引中删除
	if existing, ok := s.sessionsByID[session.ID]; ok {
		s.removeFromIndexes(existing)
	}

	s.sessionsByID[session.ID] = session
	s.addToIndexes(session)
}

// DeleteSession 删除指定 Session，并从索引中清理。
func (s *Store) DeleteSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessionsByID[id]
	if !ok {
		return
	}
	delete(s.sessionsByID, id)
	s.removeFromIndexes(session)
}

// SessionsByUser 根据 userID 返回该用户下所有 Session 的快照。
func (s *Store) SessionsByUser(userID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sessionsFromIndex(s.sessionsByUser[userID])
}

// SessionsByTenant 根据 tenantID 返回该租户下所有 Session 的快照。
func (s *Store) SessionsByTenant(tenantID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sessionsFromIndex(s.sessionsByTenant[tenantID])
}

// SessionsByDevice 根据 deviceID 返回该设备下所有 Session 的快照。
func (s *Store) SessionsByDevice(deviceID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sessionsFromIndex(s.sessionsByDevice[deviceID])
}

func (s *Store) addToIndexes(session *Session) {
	if session.UserID != "" {
		set := s.ensureIndexSet(s.sessionsByUser, session.UserID)
		set[session.ID] = struct{}{}
	}
	if session.TenantID != "" {
		set := s.ensureIndexSet(s.sessionsByTenant, session.TenantID)
		set[session.ID] = struct{}{}
	}
	if session.DeviceID != "" {
		set := s.ensureIndexSet(s.sessionsByDevice, session.DeviceID)
		set[session.ID] = struct{}{}
	}
}

func (s *Store) removeFromIndexes(session *Session) {
	if session.UserID != "" {
		s.removeFromIndexSet(s.sessionsByUser, session.UserID, session.ID)
	}
	if session.TenantID != "" {
		s.removeFromIndexSet(s.sessionsByTenant, session.TenantID, session.ID)
	}
	if session.DeviceID != "" {
		s.removeFromIndexSet(s.sessionsByDevice, session.DeviceID, session.ID)
	}
}

func (s *Store) ensureIndexSet(index map[string]map[string]struct{}, key string) map[string]struct{} {
	set, ok := index[key]
	if !ok {
		set = make(map[string]struct{})
		index[key] = set
	}
	return set
}

func (s *Store) removeFromIndexSet(index map[string]map[string]struct{}, key, id string) {
	if set, ok := index[key]; ok {
		delete(set, id)
		if len(set) == 0 {
			delete(index, key)
		}
	}
}

func (s *Store) sessionsFromIndex(index map[string]struct{}) []*Session {
	if len(index) == 0 {
		return nil
	}
	result := make([]*Session, 0, len(index))
	for id := range index {
		if session, ok := s.sessionsByID[id]; ok {
			result = append(result, session)
		}
	}
	return result
}

