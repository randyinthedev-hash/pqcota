// Package history is the append-only discovery history (규정서 §2.5⑥, §0.2 원본 불변).
// MemStore(인메모리, 테스트·단일 실행) + PgStore(Postgres 영속화)를 제공한다.
package history

import (
	"sort"
	"sync"
	"time"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
)

// Snapshot — 디스커버리 상태 스냅샷. 노드별 파생 Finding + 완전성 맵.
// 파생 뷰이므로 어떤 규칙 버전으로 만들어졌는지(RulesetVersion) 함께 보관해 재현 가능(§0.2).
type Snapshot struct {
	// Seq·CreatedAt은 적재 시 저장소가 부여한다(호출자가 채우지 않는다). 이력의 시간축 —
	// 이게 없으면 append-only로 쌓아도 "언제 무엇을 봤나"를 되짚을 수 없다.
	Seq            int64
	ID             string
	NodeID         string
	Findings       []*discoveryv1.Finding
	Edges          []*discoveryv1.ObservedEdge // 통신 엣지 관측 레인(§12, network-collector). 노드 내부 자산과 별도.
	Completeness   *commonv1.Completeness
	RulesetVersion string
	CreatedAt      time.Time

	// ExcludedByScope — 자산 스코프 정책으로 **관리 대상에서 뺀** finding 수.
	// 조용히 0으로 두면 인벤토리가 "그런 자산은 없다"고 거짓말한다 — 제외는 부재가 아니므로
	// 반드시 세어서 뷰가 고지한다(§2.7).
	ExcludedByScope int

	// Created — 이번 Append가 **새 스냅샷을 만들었는지**. false면 실질 내용이 직전과 같아
	// 기존 스냅샷을 재확인한 것이며, ID·Seq·CreatedAt은 그 기존 스냅샷의 값으로 바뀐다
	// (관측 사실 자체는 관측 기록에 언제나 남는다).
	Created bool
}

// ObsStat — 한 스냅샷이 몇 번·언제 관측됐는지. 스냅샷은 변화 시에만 쌓이므로,
// "매일 스캔했다"는 증거는 이쪽에 남는다.
type ObsStat struct {
	Count int
	First time.Time
	Last  time.Time
}

// Store — append-only 히스토리. 원본은 절대 in-place 수정하지 않는다(§0.2). 2층 구조는 설계 §13.2.
//
// 두 층으로 나뉜다:
//   - **스냅샷**(무거움) — 실질 내용이 **바뀔 때만** 쌓인다. 변화 추적·재계산 재현의 근거.
//   - **관측 기록**(가벼움) — 적재할 때마다 1건. "언제 봤나"(관측 증명)의 근거.
//
// 같은 상태를 반복 관측해도 스냅샷은 늘지 않으므로, 무거운 저장은 **변화 횟수만큼만** 자란다.
type Store interface {
	// Append — 관측 1건을 기록한다. 실질 내용이 직전과 같으면 스냅샷을 새로 만들지 않고
	// 기존 것을 가리킨다. 적재 후 s.Seq·s.CreatedAt·s.Created를 저장소가 채워 넣는다.
	Append(*Snapshot) error
	Snapshots(nodeID string) ([]*Snapshot, error)
	Latest(nodeID string) (*Snapshot, error)
	ByID(id string) (*Snapshot, error) // 스냅샷 단건(이력 상세·diff용). 없으면 (nil, nil).
	Nodes() ([]string, error)          // 스냅샷을 가진 전 노드 ID(인벤토리 뷰가 "전체"를 훑기 위함)
	// ObservationStats — 노드의 스냅샷별 관측 요약(스냅샷 id → 횟수·첫·마지막).
	ObservationStats(nodeID string) (map[string]ObsStat, error)
}

// MemStore — 인메모리 append-only 구현(테스트·단일 실행용). 영속화는 PgStore.
type MemStore struct {
	mu     sync.RWMutex
	seq    int64 // PgStore의 BIGSERIAL에 대응 — 전역 단조증가
	byNode map[string][]*Snapshot
	hash   map[string]string              // 스냅샷 id → 내용 지문
	obs    map[string]map[string]*ObsStat // node → 스냅샷 id → 관측 요약
	events []RetentionEvent               // 절단 기록(보존 정책 집행 흔적)
}

func NewMemStore() *MemStore {
	return &MemStore{
		byNode: make(map[string][]*Snapshot),
		hash:   make(map[string]string),
		obs:    make(map[string]map[string]*ObsStat),
	}
}

func (m *MemStore) Append(s *Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()

	// 실질 내용이 직전과 같으면 스냅샷을 새로 만들지 않는다 — 관측 사실만 기록.
	if prev := m.latestLocked(s.NodeID); prev != nil && m.hash[prev.ID] == ContentHash(s) {
		s.ID, s.Seq, s.CreatedAt, s.Created = prev.ID, prev.Seq, prev.CreatedAt, false
		m.observeLocked(s.NodeID, prev.ID, now)
		return nil
	}

	m.seq++
	s.Seq = m.seq
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.Created = true
	m.hash[s.ID] = ContentHash(s)
	m.byNode[s.NodeID] = append(m.byNode[s.NodeID], s)
	m.observeLocked(s.NodeID, s.ID, now)
	return nil
}

func (m *MemStore) latestLocked(nodeID string) *Snapshot {
	if s := m.byNode[nodeID]; len(s) > 0 {
		return s[len(s)-1]
	}
	return nil
}

func (m *MemStore) observeLocked(nodeID, snapID string, at time.Time) {
	if m.obs[nodeID] == nil {
		m.obs[nodeID] = make(map[string]*ObsStat)
	}
	st := m.obs[nodeID][snapID]
	if st == nil {
		st = &ObsStat{First: at}
		m.obs[nodeID][snapID] = st
	}
	st.Count++
	st.Last = at
}

func (m *MemStore) ObservationStats(nodeID string) (map[string]ObsStat, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]ObsStat, len(m.obs[nodeID]))
	for id, st := range m.obs[nodeID] {
		out[id] = *st
	}
	return out, nil
}

// ByID — 전 노드를 훑어 스냅샷 id로 찾는다(id는 전역 유일 전제).
func (m *MemStore) ByID(id string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, snaps := range m.byNode {
		for _, s := range snaps {
			if s.ID == id {
				return s, nil
			}
		}
	}
	return nil, nil
}

func (m *MemStore) Snapshots(nodeID string) ([]*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.byNode[nodeID]
	out := make([]*Snapshot, len(src))
	copy(out, src)
	return out, nil
}

func (m *MemStore) Latest(nodeID string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s := m.byNode[nodeID]; len(s) > 0 {
		return s[len(s)-1], nil
	}
	return nil, nil
}

func (m *MemStore) Nodes() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.byNode))
	for n := range m.byNode {
		out = append(out, n)
	}
	sort.Strings(out) // 결정론적 순서
	return out, nil
}
