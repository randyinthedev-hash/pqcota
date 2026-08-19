package provisioning_test

import (
	"strings"
	"testing"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	provisioningv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/provisioning/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/provisioning"
)

// Executable — §3.7 최강 게이트: FINALIZED + 승인 서명 + 조치 ≥1 만 실행 근거.
func TestExecutable(t *testing.T) {
	base := func() *provisioningv1.FinalizedPlan {
		return &provisioningv1.FinalizedPlan{
			Status:             provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED,
			ApprovalSignatures: []string{"ed25519:abc"},
			Actions:            []*provisioningv1.RemediationAction{{TargetNodeId: "web-01"}},
		}
	}

	if err := provisioning.Executable(base()); err != nil {
		t.Errorf("a finalized plan must be executable: %v", err)
	}

	// draft/in-review는 거부(§3.7).
	for _, s := range []provisioningv1.PlanStatus{
		provisioningv1.PlanStatus_PLAN_STATUS_DRAFT,
		provisioningv1.PlanStatus_PLAN_STATUS_IN_REVIEW,
		provisioningv1.PlanStatus_PLAN_STATUS_UNSPECIFIED,
	} {
		p := base()
		p.Status = s
		if err := provisioning.Executable(p); err == nil {
			t.Errorf("a status=%s plan must be refused", s)
		}
	}

	// 승인 서명 없으면 거부(§3.3③ finalize 전제).
	p := base()
	p.ApprovalSignatures = nil
	if err := provisioning.Executable(p); err == nil {
		t.Error("an unsigned plan must be refused")
	}

	// 조치 없으면 거부.
	p = base()
	p.Actions = nil
	if err := provisioning.Executable(p); err == nil {
		t.Error("an empty plan must be refused")
	}

	// nil 거부.
	if err := provisioning.Executable(nil); err == nil {
		t.Error("a nil plan must be refused")
	}
}

// ProviderClassWarnings — placeholder를 낳는 조치는 경고로 표면화되어야 한다(조용히 통과 금지).
// 단, Executable(거버넌스 게이트)은 여전히 통과시킨다 — 둘은 별개다.
func TestProviderClassWarnings(t *testing.T) {
	jca := func(kind provisioningv1.RemediationKind, choice, class string) *provisioningv1.RemediationAction {
		return &provisioningv1.RemediationAction{
			Id: "a1", TargetNodeId: "app-01", CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
			Kind: kind, ProviderChoice: choice, ProviderClass: class,
		}
	}
	plan := func(a *provisioningv1.RemediationAction) *provisioningv1.FinalizedPlan {
		return &provisioningv1.FinalizedPlan{
			Status:             provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED,
			ApprovalSignatures: []string{"r"}, Actions: []*provisioningv1.RemediationAction{a},
		}
	}
	inject := provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT

	// 알 수 없는 provider + provider_class 미지정 → placeholder → 경고 1건.
	if w := provisioning.ProviderClassWarnings(plan(jca(inject, "acme-jce", ""))); len(w) != 1 {
		t.Errorf("an undecided provider must give exactly one warning: %v", w)
	} else if !strings.Contains(w[0], "acme-jce") || !strings.Contains(w[0], "provider_class") {
		t.Errorf("the warning must name the provider and the fix: %q", w[0])
	}

	// FQCN을 계획에 넣으면 경고 없음(커스텀 provider가 계획만으로 완결).
	if w := provisioning.ProviderClassWarnings(plan(jca(inject, "acme-jce", "com.acme.AcmeProvider"))); len(w) != 0 {
		t.Errorf("with provider_class given there must be no warning: %v", w)
	}
	// 알려진 이름(BC)도 경고 없음.
	if w := provisioning.ProviderClassWarnings(plan(jca(inject, "BC", ""))); len(w) != 0 {
		t.Errorf("BC can be resolved — there must be no warning: %v", w)
	}
	// PROVIDER_INJECT가 아니면(config-only 등) 무관.
	cfg := provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY
	if w := provisioning.ProviderClassWarnings(plan(jca(cfg, "acme-jce", ""))); len(w) != 0 {
		t.Errorf("without a provider injection there must be no warning: %v", w)
	}
	// OpenSSL은 FQCN이 필요 없다 — provider 미확정이어도 경고 없음.
	ossl := jca(inject, "myprov", "")
	ossl.CryptoRuntime = commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL
	if w := provisioning.ProviderClassWarnings(plan(ossl)); len(w) != 0 {
		t.Errorf("OpenSSL needs no FQCN — there must be no warning: %v", w)
	}

	// 경고가 있어도 Executable(거버넌스)은 통과한다 — 둘은 별개 관심사.
	if err := provisioning.Executable(plan(jca(inject, "acme-jce", ""))); err != nil {
		t.Errorf("a placeholder step must still pass Executable when FINALIZED (not a hard block): %v", err)
	}
}
