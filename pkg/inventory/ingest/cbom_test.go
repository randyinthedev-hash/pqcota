package ingest_test

import (
	"testing"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/inventory/ingest"
)

const validCBOM = `{"bomFormat":"CycloneDX","specVersion":"1.6","components":[]}`

// TV-CBOM-1 (testcases.md §1), TD-SIGN-2 (testcases.md §2). CBOM 수신 어댑터.
func TestImportCBOM(t *testing.T) {
	t.Run("정상 CBOM + 바인딩 → 관측 레인 등재", func(t *testing.T) {
		disp, env, reason := ingest.ImportCBOM([]byte(validCBOM), "cmdb://node/1", nil)
		if disp != ingest.Accepted {
			t.Fatalf("disp = %v (%s), want Accepted", disp, reason)
		}
		if env.DetectionMethod != commonv1.DetectionMethod_DETECTION_METHOD_ARTIFACT {
			t.Errorf("detection_method = %v, want ARTIFACT (관측 레인)", env.DetectionMethod)
		}
		if env.TargetNodeId != "cmdb://node/1" {
			t.Errorf("target_node_id = %q", env.TargetNodeId)
		}
	})

	t.Run("스키마 부적합 → 거부", func(t *testing.T) {
		if disp, _, _ := ingest.ImportCBOM([]byte(`{"bomFormat":"SPDX"}`), "cmdb://node/1", nil); disp != ingest.Rejected {
			t.Errorf("disp = %v, want Rejected", disp)
		}
		if disp, _, _ := ingest.ImportCBOM([]byte(`not json`), "cmdb://node/1", nil); disp != ingest.Rejected {
			t.Errorf("malformed: disp = %v, want Rejected", disp)
		}
	})

	t.Run("바인딩 없음 → 스코프 판정 요청", func(t *testing.T) {
		if disp, _, _ := ingest.ImportCBOM([]byte(validCBOM), "", nil); disp != ingest.NeedsScopeBinding {
			t.Errorf("disp = %v, want NeedsScopeBinding", disp)
		}
	})

	t.Run("서명 불일치(변조) → 거부", func(t *testing.T) {
		reject := func([]byte) bool { return false } // 서명 검증 실패 시뮬레이트
		if disp, _, _ := ingest.ImportCBOM([]byte(validCBOM), "cmdb://node/1", reject); disp != ingest.Rejected {
			t.Errorf("disp = %v, want Rejected (변조 방지)", disp)
		}
	})
}
