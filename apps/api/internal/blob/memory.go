package blob

import (
	"context"
	"fmt"
	"sync"
)

type MemoryStore struct {
	mu    sync.RWMutex
	store map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string][]byte),
	}
}

func (m *MemoryStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Copy data to prevent mutation
	copied := make([]byte, len(data))
	copy(copied, data)
	m.store[key] = copied
	return nil
}

func (m *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.store[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Copy to prevent external mutation
	copied := make([]byte, len(data))
	copy(copied, data)
	return copied, nil
}

func (m *MemoryStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.store, key)
	return nil
}
