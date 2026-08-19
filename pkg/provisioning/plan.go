package provisioning

import (
	"errors"
	"fmt"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	provisioningv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/provisioning/v1"
)

// ErrNotFinalized — FINALIZED 아닌 계획을 실행 근거로 쓰려 할 때(§3.7 최강 게이트).
var ErrNotFinalized = errors.New("plan not finalized — refusing to provision (§3.7)")

// Executable — 확정 계획이 프로비저닝 실행 근거로 유효한지 검증한다(§3.7 Inventory→Deploy 게이트).
// 규칙: FINALIZED 상태 + 승인 서명(§3.3③) + 조치 최소 1건. 파생이 아니라 실행 직전 관문이다.
//
// ★ 경계(§5): 이 함수는 전 컴포넌트가 공유하는 "실행 근거" 계약 규칙일 뿐이다.
// 실제 단계적 실행·오케스트레이션(drain·rolling·게이트·롤백)은 하지 않는다(§4.3).
func Executable(p *provisioningv1.FinalizedPlan) error {
	if p == nil {
		return fmt.Errorf("%w: nil", ErrNotFinalized)
	}
	if p.GetStatus() != provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED {
		return fmt.Errorf("%w: status=%s", ErrNotFinalized, p.GetStatus())
	}
	if len(p.GetApprovalSignatures()) == 0 {
		return fmt.Errorf("%w: no approval signature", ErrNotFinalized)
	}
	if len(p.GetActions()) == 0 {
		return fmt.Errorf("%w: no actions", ErrNotFinalized)
	}
	return nil
}

// ProviderClassWarnings — 계획을 훑어, JCA provider 주입인데 provider_class를 확정할 수 없어
// java.security 조각에 **placeholder가 들어가는** 조치를 찾아 경고로 돌린다.
//
// ★ 이건 Executable(거버넌스 게이트)이 **아니다** — 계획은 유효하고 플레이북도 정상 생성된다.
// placeholder는 의도된 정직 경로다(FQCN을 추측하지 않고 사람이 채운다 — jca.go). 다만 산출물이
// 그대로는 불완전하므로, 도구가 조용히 통과시키지 않도록 호출부(pqcota-provision)가 이걸 stderr에
// 크게 알린다(§2.5 — 불명을 삼키지 않는다). 하드 블록은 "생성→사람이 FQCN 기입→적용"이라는
// 정당한 워크플로를 막으므로 하지 않는다.
// ProviderSlotWarnings — provider 주입은 java.security의 **한 자리를 대체**한다. 조각 안 주석은
// 열어봐야 보이므로, 무엇이 밀려나는지 여기서 크게 알린다(§2.6 — 유실을 조용히 두지 않는다).
func ProviderSlotWarnings(p *provisioningv1.FinalizedPlan) []string {
	var out []string
	for _, a := range p.GetActions() {
		if a.GetKind() != provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT ||
			a.GetCryptoRuntime() != commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA {
			continue
		}
		out = append(out, fmt.Sprintf("조치 %s(node=%s): `security.provider.2`는 그 자리를 **대체한다** — 원래 2번이던 provider(JDK 기본이면 대개 SunRsaSign)가 목록에서 빠지고 해당 서비스가 새 provider로 넘어간다. 밀어내지 않으려면 대상의 java.security에서 뒤 번호를 미룬 뒤 넣을 것.",
			a.GetId(), a.GetTargetNodeId()))
	}
	return out
}

func ProviderClassWarnings(p *provisioningv1.FinalizedPlan) []string {
	var out []string
	for _, a := range p.GetActions() {
		if a.GetKind() != provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT {
			continue
		}
		if a.GetCryptoRuntime() != commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA {
			continue // OpenSSL은 모듈 경로만 있으면 됨 — FQCN 불필요.
		}
		if _, exact := providerClass(a.GetProviderChoice(), a.GetProviderClass()); !exact {
			out = append(out, fmt.Sprintf(
				"조치 %s(node=%s, provider=%q): provider_class 미확정 — java.security 조각에 placeholder가 들어간다. 계획의 provider_class에 FQCN을 넣거나 배포 전 조각의 클래스명을 교체할 것.",
				a.GetId(), a.GetTargetNodeId(), a.GetProviderChoice()))
		}
	}
	return out
}
