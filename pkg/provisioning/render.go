package provisioning

import (
	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	provisioningv1 "github.com/pqcota/pqcota/gen/pqcota/provisioning/v1"
	"github.com/pqcota/pqcota/pkg/kernel/registry"
)

// Render — 확정 계획의 조치 한 건에 대한 배포 아티팩트(config 조각)를 결정론적으로 생성한다(§0.2 파생).
// crypto_runtime으로 OpenSSL/JCA 분기(docs/암호_런타임_수용_원칙.md). config 주입형(CONFIG_ONLY·PROVIDER_INJECT)은 실제 조각을,
// 그 외(포크 교체·프록시·재빌드·JDK 업그레이드·앱 reconfig·폐기)는 config로 안 되는 조치임을
// 정직하게 명시한 주석 블록을 낸다(§4.3/§4.4 "레거시 터치").
//
// ★ 경계: 생성만 한다 — 배치·활성화·재시작·오케스트레이션은 하지 않는다(§4.5).
func Render(a *provisioningv1.RemediationAction) string {
	switch a.GetCryptoRuntime() {
	case commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL:
		return renderOpenSSL(a)
	case commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA:
		return renderJCA(a)
	default:
		return "# (미상 런타임) — 조치 아티팩트 생성 불가\n"
	}
}

// FillPlan — 계획의 모든 조치에 config_artifact를 채운다(리뷰 대상 diff 실체화, §3 정책 템플릿).
// 파생이므로 항상 재생성 가능(§0.2) — 저장은 편의일 뿐 원본은 (kind·target·provider).
func FillPlan(p *provisioningv1.FinalizedPlan) {
	for _, a := range p.GetActions() {
		a.ConfigArtifact = Render(a)
	}
}

// hybridGroup — 목표 알고리즘(KEM)에서 TLS 하이브리드 그룹 wire 이름을 뽑는다.
// 서명 알고리즘(ML-DSA 등)·미상이면 "" (KEM 그룹 해당 없음).
func hybridGroup(target string) string {
	a, ok := registry.MatchPQC(target)
	if !ok || a.Kind != registry.KindKEM {
		return ""
	}
	switch a.Family {
	case "ML-KEM":
		return "X25519MLKEM768" // OpenSSL 3.5+ / BCJSSE 하이브리드 그룹명
	case "Kyber":
		return "X25519Kyber768Draft00" // 전신(초안)
	default:
		return "" // 그 외 KEM은 표준 하이브리드 그룹명 미고정 — 조각에서 주석 처리
	}
}
