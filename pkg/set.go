package pkg

import "sync"

type Set struct {
	mu sync.RWMutex
	m  map[string]interface{}
}

func NewSet() *Set {
	return &Set{
		m: make(map[string]interface{}),
	}
}

func (s *Set) Contains(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.m[key]; ok {
		return true
	}
	return false
}

func (s *Set) Add(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

func (s *Set) Value(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.m[key]; ok {
		return v, true
	}
	return nil, false
}

func (s *Set) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.m[key]; ok {
		return v, true
	}
	return nil, false
}

func (s *Set) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}
