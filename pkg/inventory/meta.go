package inventory

import (
	"sync"

	inventoryv1 "github.com/pqcota/pqcota/gen/pqcota/inventory/v1"
)

// MetaStore — 머신 메타데이터(엔드포인트·프로필) 저장소. 히스토리(append-only)와 달리 **upsert**한다
// — 엔드포인트·프로필은 사용자가 재사용·수정하는 가변 메타데이터(§2.0). 접근 비밀은 담기지 않는다.
type MetaStore interface {
	UpsertEndpoint(*inventoryv1.MachineEndpoint) error
	UpsertProfile(*inventoryv1.MachineProfile) error
	Endpoint(nodeID string) (*inventoryv1.MachineEndpoint, error)
	Profile(nodeID string) (*inventoryv1.MachineProfile, error)
}

// MemMetaStore — 인메모리 구현(테스트·단일 실행).
type MemMetaStore struct {
	mu   sync.RWMutex
	ep   map[string]*inventoryv1.MachineEndpoint
	prof map[string]*inventoryv1.MachineProfile
}

func NewMemMetaStore() *MemMetaStore {
	return &MemMetaStore{ep: map[string]*inventoryv1.MachineEndpoint{}, prof: map[string]*inventoryv1.MachineProfile{}}
}

func (m *MemMetaStore) UpsertEndpoint(e *inventoryv1.MachineEndpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ep[e.GetNodeId()] = e
	return nil
}

func (m *MemMetaStore) UpsertProfile(p *inventoryv1.MachineProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prof[p.GetNodeId()] = p
	return nil
}

func (m *MemMetaStore) Endpoint(nodeID string) (*inventoryv1.MachineEndpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ep[nodeID], nil
}

func (m *MemMetaStore) Profile(nodeID string) (*inventoryv1.MachineProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.prof[nodeID], nil
}
