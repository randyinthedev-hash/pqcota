package scope_test

import (
	"reflect"
	"testing"

	"github.com/pqcota/pqcota/pkg/kernel/scope"
)

// TD-SCOPE-1 (testcases.md §2). 스코프 게이트·라우팅.
func TestScopeGate(t *testing.T) {
	m := scope.NewMaster([]string{"node-a", "node-b"})

	t.Run("미등재 노드는 수집 대상에서 필터 제외", func(t *testing.T) {
		allowed, rejected := m.Gate([]string{"node-a", "node-x", "node-b"})
		if !reflect.DeepEqual(allowed, []string{"node-a", "node-b"}) {
			t.Errorf("allowed = %v, want [node-a node-b]", allowed)
		}
		if !reflect.DeepEqual(rejected, []string{"node-x"}) {
			t.Errorf("rejected = %v, want [node-x]", rejected)
		}
	})

	t.Run("미등재 관측 노드 → 등재 판정 요청(수집 안 함)", func(t *testing.T) {
		if got := m.ClassifyObserved("node-x"); got != scope.RegistrationRequest {
			t.Errorf("ClassifyObserved(unregistered) = %v, want RegistrationRequest", got)
		}
		if got := m.ClassifyObserved("node-a"); got != scope.InScope {
			t.Errorf("ClassifyObserved(registered) = %v, want InScope", got)
		}
	})
}
