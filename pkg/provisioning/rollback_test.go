package provisioning_test

import (
	"strings"
	"testing"

	provisioningv1 "github.com/pqcota/pqcota/gen/pqcota/provisioning/v1"
	"github.com/pqcota/pqcota/pkg/provisioning"
)

// L2 롤백: forward가 배치한 config 조각 + 스테이지한 모듈을 제거(state: absent). 재시작은 하지 않는다 — 그건 L3.
func TestRollbackPlaybookL2(t *testing.T) {
	pb := provisioning.GenerateRollbackPlaybook(samplePlan(), provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL)
	for _, want := range []string{
		`hosts: ["web-01"]`, `hosts: ["app-01"]`,
		"state: absent",        // 제거(역방향)
		"/opt/pqcota/BC.jar",   // a2 주입 모듈 제거
		"java.security.pqcota", // a2 JCA config 조각 제거
		"openssl-pqc.cnf",      // a1 OpenSSL config 조각 제거
		"a3(REMEDIATION_KIND_FORK_REPLACE): config로 배포하지 않았으므로 롤백도 수동", // 비-config는 수동
	} {
		if !strings.Contains(pb, want) {
			t.Errorf("L2 롤백 플레이북에 %q 없음:\n%s", want, pb)
		}
	}
	if strings.Contains(pb, "restart") || strings.Contains(pb, "재시작:") {
		t.Errorf("롤백에 재시작이 있으면 안 됨(재시작은 L3의 restart 훅):\n%s", pb)
	}
	if strings.Contains(pb, "ansible.builtin.copy") {
		t.Errorf("롤백은 배치(copy)가 아니라 제거(absent)여야:\n%s", pb)
	}
}

// L1 롤백: 스테이지한 모듈만 제거 — config 조각은 L1에서 배치 안 했으니 제거도 없음.
func TestRollbackPlaybookL1(t *testing.T) {
	pb := provisioning.GenerateRollbackPlaybook(samplePlan(), provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L1_STAGE_ONLY)
	if !strings.Contains(pb, "/opt/pqcota/BC.jar") {
		t.Error("L1 롤백도 스테이지한 모듈은 제거해야")
	}
	if strings.Contains(pb, "config 조각 제거") {
		t.Errorf("L1(stage-only)은 config를 배치 안 했으니 config 제거도 없어야:\n%s", pb)
	}
}
