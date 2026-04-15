package yaml

import "sync"

type Store struct {
	mu   sync.RWMutex
	docs map[string][]*Document
}

func NewStore() *Store {
	return &Store{docs: make(map[string][]*Document)}
}

func (s *Store) Update(uri, content string) {
	docs := ParseFile(content)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(docs) > 0 {
		s.docs[uri] = docs
	}
}

func (s *Store) Get(uri string) []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

func (s *Store) Remove(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

func (s *Store) URIs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uris := make([]string, 0, len(s.docs))
	for uri := range s.docs {
		uris = append(uris, uri)
	}
	return uris
}
