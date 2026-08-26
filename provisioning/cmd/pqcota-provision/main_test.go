package main_test

// TP-PLAN-GATE — §3.7 실행 게이트가 **CLI에서** 실제로 막나.
//
// 규칙 자체는 pkg/provisioning/plan_test.go가 이미 덮는다. 여기서 보는 것은 **배선**이다.
// 규칙이 옳아도 제품 경로가 부르지 않으면 보장이 아니고, 실제로 한동안 그랬다: CLI가
// Executable을 부르지 않고 상태 비교만 인라인으로 해서, 승인 서명이나 조치가 빈 FINALIZED
// 계획도 플레이북을 받아 갔다.
//
// 그래서 함수가 아니라 **빌드한 CLI를 돌려서** 본다. 반환값을 버리는 식으로 배선이 헐거워져도
// 여기서 드러난다.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildCLI — 테스트 안에서 CLI를 빌드한다. 산출물은 t.TempDir이라 뒤에 남지 않는다.
func buildCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "pqcota-provision")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func writePlan(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	// 정상 — 견본(examples/provisioning/plans/openssl-3.5-config-only.json)과 같은 모양.
	planOK = `{
  "id": "t-ok", "status": "PLAN_STATUS_FINALIZED", "scope": "ring-0",
  "approvalSignatures": ["reviewer:test"],
  "actions": [{"id":"a1","targetNodeId":"n1","findingId":"f1",
    "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL","kind":"REMEDIATION_KIND_CONFIG_ONLY"}]
}`
	// 승인 서명이 없다 — FINALIZED이지만 실행 근거가 아니다(§3.3③).
	planNoSig = `{
  "id": "t-nosig", "status": "PLAN_STATUS_FINALIZED", "scope": "ring-0",
  "actions": [{"id":"a1","targetNodeId":"n1","findingId":"f1",
    "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL","kind":"REMEDIATION_KIND_CONFIG_ONLY"}]
}`
	// 조치가 없다 — 바꿀 것이 없는데 플레이북을 내면 빈 산출물이 실행 근거처럼 보인다.
	planNoActions = `{
  "id": "t-noact", "status": "PLAN_STATUS_FINALIZED", "scope": "ring-0",
  "approvalSignatures": ["reviewer:test"]
}`
	// 확정되지 않았다 — 가장 바깥 게이트.
	planDraft = `{
  "id": "t-draft", "status": "PLAN_STATUS_DRAFT", "scope": "ring-0",
  "approvalSignatures": ["reviewer:test"],
  "actions": [{"id":"a1","targetNodeId":"n1","findingId":"f1",
    "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL","kind":"REMEDIATION_KIND_CONFIG_ONLY"}]
}`
)

// TestPlanGateRefuses — 실행 근거가 아닌 계획은 **거절되고 플레이북이 한 줄도 나오지 않는다.**
// 종료 코드만 보면 모자란다: 거절하면서 산출물을 함께 내면 그것을 받아 돌리는 사람이 생긴다.
func TestPlanGateRefuses(t *testing.T) {
	bin := buildCLI(t)
	for _, tc := range []struct {
		name, plan, want string
	}{
		{"승인 서명이 없다", planNoSig, "no approval signature"},
		{"조치가 없다", planNoActions, "no actions"},
		{"확정되지 않았다", planDraft, "PLAN_STATUS_DRAFT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, args := range [][]string{
				{"--level", "l2"},
				{"--level", "l2", "--rollback"}, // 롤백 경로도 같은 게이트를 지난다
			} {
				cmd := exec.Command(bin, append(args, writePlan(t, tc.plan))...)
				var stdout, stderr strings.Builder
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()

				if err == nil {
					t.Fatalf("%v: 통과했다 — 게이트가 없다", args)
				}
				if out := strings.TrimSpace(stdout.String()); out != "" {
					t.Errorf("%v: 거절하면서 stdout에 산출물을 냈다:\n%s", args, out)
				}
				if !strings.Contains(stderr.String(), tc.want) {
					t.Errorf("%v: 무엇이 모자란지 말하지 않는다 (%q 없음):\n%s", args, tc.want, stderr.String())
				}
			}
		})
	}
}

// TestPlanGateAllows — 게이트가 정상 계획까지 막으면 그것도 결함이다.
func TestPlanGateAllows(t *testing.T) {
	bin := buildCLI(t)
	cmd := exec.Command(bin, "--level", "l2", writePlan(t, planOK))
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("정상 계획이 거절됐다: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "hosts:") {
		t.Errorf("플레이북이 나오지 않았다:\n%s", stdout.String())
	}
}
