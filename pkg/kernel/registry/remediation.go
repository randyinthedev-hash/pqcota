package registry

// RemediationAction — 관측된 암호 협상에 대한 권고 조치(remediation 분기).
type RemediationAction string

const (
	ActionNone    RemediationAction = "none"    // 표준 PQC — 유지(규제 자산은 FIPS provider 확인만)
	ActionMigrate RemediationAction = "migrate" // 고전(비PQC) → PQC 하이브리드 도입
	ActionUpgrade RemediationAction = "upgrade" // 표준 전신(초안 PQC) → 최종 표준으로 상향
	ActionReplace RemediationAction = "replace" // 실험/파훼 PQC → 표준으로 교체
)

// Remediation — 성숙도(+규제 여부)에서 파생한 권고. Priority는 0(불요)~4(즉시).
type Remediation struct {
	Action    RemediationAction
	Priority  int    // 0 없음 · 1 확인 · 2 계획 · 3 시급 · 4 즉시(파훼)
	Target    string // 권장 목표 표준(KEM→ML-KEM, 서명→ML-DSA)
	Rationale string
}

// standardTargetFor — 종류별 최종 표준 목표. remediation의 도착지.
func standardTargetFor(k PQCKind) string {
	if k == KindSignature {
		return "ML-DSA (FIPS 204)"
	}
	return "ML-KEM (FIPS 203)" // KEM 기본
}

// Remediate — 이미 PQC로 식별된 협상의 성숙도 기반 권고.
// 규제 자산(regulated)은 표준이어도 FIPS 검증 provider 사용을 확인해야 한다.
func (a PQCAlgorithm) Remediate(regulated bool) Remediation {
	target := standardTargetFor(a.Kind)
	switch a.Maturity {
	case MaturityFIPS:
		if regulated {
			return Remediation{ActionNone, 1, target, "표준 PQC — 규제 자산: FIPS 검증 provider 사용 확인"}
		}
		return Remediation{ActionNone, 0, target, "표준 PQC — 조치 불요"}
	case MaturityDraft:
		return Remediation{ActionUpgrade, 2, target, a.Family + "은(는) 표준 전신(초안) — 최종 표준으로 상향"}
	case MaturityExperimental:
		return Remediation{ActionReplace, 3, target, a.Family + "은(는) 비표준 실험 알고리즘 — 표준으로 교체"}
	case MaturityBroken:
		return Remediation{ActionReplace, 4, target, a.Family + "은(는) 파훼됨 — 즉시 교체"}
	default:
		return Remediation{ActionNone, 0, target, ""}
	}
}
