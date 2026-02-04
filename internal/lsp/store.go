package lsp

import "sync"

type Store struct {
	mu   sync.RWMutex
	docs map[string]string // uri -> text
}

func NewStore() *Store {
	return &Store{docs: map[string]string{}}
}

func (s *Store) Set(uri, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[uri] = text
}

func (s *Store) Get(uri string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.docs[uri]
	return t, ok
}

func (s *Store) Delete(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}
