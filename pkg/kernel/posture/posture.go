// Package posture classifies a negotiated key-exchange group into a quantum-resistance
// posture (규정서 §12.1). 파생 뷰 — 관측된 협상 그룹 문자열에서 결정론적으로 판정한다(§0.2).
// 공유 규칙: network-collector도, 대조 엔진도 같은 규칙을 써야 하므로 여기에 둔다.
package posture

import (
	"strings"

	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/kernel/registry"
)

// pqcTokens — 협상 그룹 이름에 이 토큰이 들어가면 PQC(하이브리드 포함)로 본다(🟢).
// 하이브리드(X25519MLKEM768)든 순수 PQC(mlkem768)든 posture는 동일하게 PQC로 표기한다(§12.1).
var pqcTokens = []string{
	"MLKEM", "KYBER", // ML-KEM (FIPS 203) / 구 Kyber
	"SNTRUP",      // sntrup761 (OpenSSH)
	"BIKE", "HQC", // NIST 라운드4 KEM
	"FRODO", "MCELIECE", // 격자/코드 기반
	"MLDSA", "DILITHIUM", "FALCON", "SPHINCS", "SLHDSA", // PQC 서명(SSH 호스트키 등)
}

// classicalTokens — 순수 고전(양자 취약) 협상(🔴).
var classicalTokens = []string{
	"X25519", "CURVE25519", "X448",
	"ECDH", "ECDHE", "SECP", "PRIME", "NISTP", "BRAINPOOL",
	"FFDHE", "DHE", "DIFFIE",
	"RSA",
}

// Classify — 협상된 KEX 그룹(+cipher 보조)에서 양자내성 posture를 판정한다.
//   - PQC 토큰 포함           → PQC_HYBRID (🟢). 하이브리드 X25519MLKEM768도 여기(고전+PQC 혼합이라도 PQC 보호).
//   - 그룹 미상(빈 문자열)      → UNSPECIFIED (⚪ 불명·미관측). "고전"으로 단정하지 않는다(§12.2 정직성).
//   - 고전 토큰만              → CLASSICAL (🔴).
//   - 알 수 없는 그룹          → UNSPECIFIED (⚪). 임의 추정 금지.
func Classify(negotiatedGroup, cipher string) discoveryv1.QuantumPosture {
	g := strings.ToUpper(strings.TrimSpace(negotiatedGroup))
	if g == "" {
		return discoveryv1.QuantumPosture_QUANTUM_POSTURE_UNSPECIFIED // ⚪ 관측 못 함 ≠ 고전
	}
	for _, t := range pqcTokens {
		if strings.Contains(g, t) {
			return discoveryv1.QuantumPosture_QUANTUM_POSTURE_PQC_HYBRID
		}
	}
	for _, t := range classicalTokens {
		if strings.Contains(g, t) {
			return discoveryv1.QuantumPosture_QUANTUM_POSTURE_CLASSICAL
		}
	}
	return discoveryv1.QuantumPosture_QUANTUM_POSTURE_UNSPECIFIED // 미지의 그룹 → 불명
}

// Grade — PQC 그룹의 표준화 성숙도(표준/초안/실험/취약). PQC가 아니면(고전·불명) "".
// posture의 "PQC vs 고전" 축에 "표준 vs 실험" 축을 더한다 — 규제 자산 라우팅·remediation 분기(§4.10).
func Grade(negotiatedGroup string) registry.PQCMaturity {
	if a, ok := registry.MatchPQC(negotiatedGroup); ok {
		return a.Maturity
	}
	return ""
}

// GradeLabel — 성숙도 한글 라벨(표시용). 빈 성숙도는 "".
func GradeLabel(m registry.PQCMaturity) string {
	switch m {
	case registry.MaturityFIPS:
		return "표준"
	case registry.MaturityDraft:
		return "초안"
	case registry.MaturityExperimental:
		return "실험"
	case registry.MaturityBroken:
		return "취약"
	default:
		return ""
	}
}

// Recommend — 관측된 협상 그룹 하나에 대한 remediation 권고(§4.10).
// PQC면 성숙도로 분기(registry.Remediate), 고전이면 마이그레이션, 미관측이면 보류.
//   - PQC(표준/초안/실험/파훼) → registry.Remediate: 유지/상향/교체/즉시
//   - CLASSICAL(🔴)           → migrate: 양자취약, PQC 하이브리드 도입
//   - UNSPECIFIED(⚪)         → none: 미관측 — 판단 보류(§12.2 정직성)
func Recommend(negotiatedGroup, cipher string, regulated bool) registry.Remediation {
	if a, ok := registry.MatchPQC(negotiatedGroup); ok {
		return a.Remediate(regulated)
	}
	switch Classify(negotiatedGroup, cipher) {
	case discoveryv1.QuantumPosture_QUANTUM_POSTURE_CLASSICAL:
		prio := 3
		if regulated {
			prio = 4 // 규제 자산의 고전 협상은 최우선
		}
		return registry.Remediation{
			Action:    registry.ActionMigrate,
			Priority:  prio,
			Target:    "ML-KEM 하이브리드 (FIPS 203)",
			Rationale: "고전 키교환 — 양자취약(HNDL), PQC 하이브리드로 마이그레이션",
		}
	default:
		return registry.Remediation{Action: registry.ActionNone, Priority: 0, Rationale: "협상 미관측 — 판단 보류"}
	}
}

// Symbol — posture의 대시보드 기호(§12.1). 토폴로지 범례·요약에 쓴다.
func Symbol(p discoveryv1.QuantumPosture) string {
	switch p {
	case discoveryv1.QuantumPosture_QUANTUM_POSTURE_PQC_HYBRID:
		return "🟢"
	case discoveryv1.QuantumPosture_QUANTUM_POSTURE_CLASSICAL:
		return "🔴"
	default:
		return "⚪"
	}
}
