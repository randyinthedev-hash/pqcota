package normalize_test

import (
	"testing"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	"github.com/pqcota/pqcota/pkg/discovery/normalize"
)

// TD-GAP-1 (디스커버리_테스트케이스.md §2). 완전성 맵 — 갭 ≠ 부재.
func TestCompleteness(t *testing.T) {
	proc := commonv1.CollectionLayer_COLLECTION_LAYER_PROCESS
	arti := commonv1.CollectionLayer_COLLECTION_LAYER_ARTIFACT

	t.Run("PROCESS 선언·미커버 → 갭 기록", func(t *testing.T) {
		c := normalize.BuildCompleteness(
			[]commonv1.CollectionLayer{proc, arti}, // 커버 가능하다고 선언
			[]commonv1.CollectionLayer{arti},       // 실제로는 artifact만 (프로세스 미실행)
			"프로세스 계층 미수집 — 대상 미실행",
		)
		if len(c.LayersMissing) != 1 || c.LayersMissing[0] != proc {
			t.Fatalf("layers_missing = %v, want [PROCESS]", c.LayersMissing)
		}
		if c.Note == "" {
			t.Error("갭 사유(note)가 비어 있으면 안 됨 — 자동 '부재' 처리 금지(§2.6)")
		}
	})

	t.Run("전부 커버 → 갭 없음", func(t *testing.T) {
		c := normalize.BuildCompleteness(
			[]commonv1.CollectionLayer{proc, arti},
			[]commonv1.CollectionLayer{arti, proc},
			"",
		)
		if len(c.LayersMissing) != 0 {
			t.Fatalf("layers_missing = %v, want empty", c.LayersMissing)
		}
	})
}
