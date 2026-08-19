// Package cng — Windows CNG(Cryptography Next Generation) collector.
//
// **왜 별도 collector인가** — CNG는 provider(KSP/SSP) 아키텍처라 JCA와 동형이다(수용 원칙 §2.1).
// 등록된 provider 목록이 곧 능력의 경계이고 **순서가 우선순위**라, JCA collector가 provider 체인을
// 보는 것과 같은 축을 본다. 다만 수집 수단은 하나도 겹치지 않는다 — `/proc`도 ELF도 attach도 아닌
// `bcrypt.dll`의 열거 API다(검토 중인 설계 §2.2).
//
// **관측까지만 한다.** evidence_strength·pqc_readiness 같은 파생은 코어가 만든다(§1.2).
package cng

import "errors"

// ErrNotWindows — Windows가 아닌 곳에서 관측을 요청했다. **"CNG 없음"이 아니라 "관측 불가"다**(§2.6).
var ErrNotWindows = errors.New("CNG는 Windows에서만 관측할 수 있다 — 없는 것이 아니라 여기서 못 보는 것이다")

// Observation — 한 노드의 CNG 관측 결과.
//
// 순수 데이터라 Windows 없이도 만들고 검사할 수 있다 — 실물 없이 단위 테스트되게 파싱·조립을
// I/O에서 뗀 것이다.
type Observation struct {
	// Providers — 등록된 provider 이름. **순서를 보존한다**(우선순위 협상의 근거, 수용 원칙 §2.2).
	Providers []string `json:"providers"`
	// Algorithms — 열거된 알고리즘. 계약의 CngAxes에는 아직 자리가 없어 파생 뷰로는 가지 않지만,
	// 원본(raw_capture)에는 그대로 실어 보낸다 — 관측한 것을 버리지 않는다(§1.2 재계산 가능).
	Algorithms []Algorithm `json:"algorithms,omitempty"`
}

// Algorithm — CNG가 열거한 알고리즘 하나와 그 종류.
type Algorithm struct {
	Name  string `json:"name"`
	Class string `json:"class"` // cipher · hash · asymmetric-encryption · secret-agreement · signature · rng · key-derivation
}

// Empty — 아무것도 관측하지 못했나. 빈 관측과 "provider가 없다"는 다르다 — 부르는 쪽이 갭으로
// 적을지 판단할 수 있게 관측 자체의 공백만 답한다.
func (o Observation) Empty() bool { return len(o.Providers) == 0 && len(o.Algorithms) == 0 }
