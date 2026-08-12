package history

import (
	"errors"
	"testing"

	"github.com/pqcota/pqcota/pkg/org"
)

// TestStoreIsBoundToAnOrg — 저장소를 열면 반드시 어떤 조직에 묶인다. 안 묶인 저장소는 만들 수 없다.
//
// 조직이 없는 핸들을 만들 수 있으면 그 핸들이 쓴 행은 나중에 누구 것인지 알 수 없다.
func TestStoreIsBoundToAnOrg(t *testing.T) {
	if got := NewMemStore().Org(); got != org.Default {
		t.Fatalf("조직 없이 연 저장소가 %q에 묶였다 — org.Default여야 한다", got)
	}
	m, err := NewMemStoreIn("acme")
	if err != nil || m.Org() != org.ID("acme") {
		t.Fatalf("조직을 대고 열었는데 %q %v", m.Org(), err)
	}
	if _, err := NewMemStoreIn("Acme"); err == nil {
		t.Fatal("모양이 틀린 조직을 받아들였다 — 오타가 조용히 다른 조직이 된다")
	}
}

// TestRequiredModeRefusesTheDefaultStore — 여럿을 담는 배포에서 조직 없는 저장소는 열리지 않는다.
//
// 데이터가 섞인 뒤에는 되돌릴 수 없으므로, 쓰는 자리가 아니라 **여는 자리**에서 막는다.
func TestRequiredModeRefusesTheDefaultStore(t *testing.T) {
	t.Setenv(org.RequireEnv, "1")
	if _, err := NewMemStoreIn(""); !errors.Is(err, org.ErrDefaultNotAllowed) {
		t.Fatalf("필수 모드인데 조직 없는 저장소가 열렸다: %v", err)
	}
}

// TestOrgsDoNotSeeEachOther — 같은 node_id를 쓰는 두 조직이 서로의 이력을 보지 않는다.
//
// `web-01` 충돌은 예외가 아니라 기본값에 가깝다 — node_id는 고객 CMDB ID·FQDN·machine-id 파생
// self-id이기 때문이다. 섞이면 한 노드의 이력으로 **병합되어** 되돌릴 수 없다.
func TestOrgsDoNotSeeEachOther(t *testing.T) {
	a, err := NewMemStoreIn("acme")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewMemStoreIn("beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Append(&Snapshot{ID: "snap-a", NodeID: "web-01", RulesetVersion: "r1"}); err != nil {
		t.Fatal(err)
	}

	if nodes, _ := b.Nodes(); len(nodes) != 0 {
		t.Fatalf("다른 조직의 노드가 보인다: %v", nodes)
	}
	if s, _ := b.ByID("snap-a"); s != nil {
		t.Fatal("ID만 알면 남의 스냅샷이 열린다")
	}
	if s, _ := b.Latest("web-01"); s != nil {
		t.Fatal("같은 node_id로 남의 최신 스냅샷이 보인다")
	}
	if nodes, _ := a.Nodes(); len(nodes) != 1 {
		t.Fatalf("자기 조직 노드가 안 보인다: %v", nodes)
	}
}
