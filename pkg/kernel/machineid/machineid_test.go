package machineid_test

import (
	"testing"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	"github.com/pqcota/pqcota/pkg/kernel/machineid"
)

// SelfAssign은 결정론적(같은 지문→같은 id)이고 우선순위를 지킨다.
func TestSelfAssign(t *testing.T) {
	fp := &commonv1.MachineIdentity{MachineId: "abc-123", Fqdn: "web-01"}
	id1, src1 := machineid.SelfAssign(fp)
	id2, _ := machineid.SelfAssign(fp)
	if id1 == "" || id1 != id2 {
		t.Fatalf("결정론 실패: %q vs %q", id1, id2)
	}
	if src1 != "machine-id" { // fqdn보다 우선
		t.Errorf("우선순위 오류: %s (machine-id여야)", src1)
	}

	// cloud-instance-id가 최우선
	fp.CloudInstanceId = "i-0999"
	if _, src := machineid.SelfAssign(fp); src != "cloud-instance-id" {
		t.Errorf("cloud 우선 실패: %s", src)
	}

	// 다른 머신 → 다른 id
	other, _ := machineid.SelfAssign(&commonv1.MachineIdentity{MachineId: "xyz-789"})
	base, _ := machineid.SelfAssign(&commonv1.MachineIdentity{MachineId: "abc-123"})
	if other == base {
		t.Error("다른 machine-id가 같은 id로")
	}

	// 지문 없음 → 빈 값
	if id, _ := machineid.SelfAssign(&commonv1.MachineIdentity{}); id != "" {
		t.Errorf("지문 없으면 빈 id여야: %q", id)
	}

	// 네임스페이스 접두
	if len(id1) < 6 || id1[:5] != "node:" {
		t.Errorf("node: 접두 없음: %q", id1)
	}
}
