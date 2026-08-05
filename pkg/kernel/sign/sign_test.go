package sign_test

import (
	"testing"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/kernel/sign"
)

func result(node, cbom string) *discoveryv1.CollectionResult {
	return &discoveryv1.CollectionResult{
		Envelope:      &commonv1.Envelope{TargetNodeId: node},
		CbomCyclonedx: []byte(cbom),
		RawFormat:     "test/v1",
	}
}

func TestSignVerify(t *testing.T) {
	pub, priv, err := sign.Generate()
	if err != nil {
		t.Fatal(err)
	}
	res := result("web-01", `{"a":1}`)
	sig, err := sign.Sign(priv, res)
	if err != nil {
		t.Fatal(err)
	}
	res.Envelope.Signature = sig

	if !sign.Verify([]string{pub}, res) {
		t.Error("정상 서명이 검증 실패")
	}

	// 변조(CBOM 변경) → 검증 실패.
	res.CbomCyclonedx = []byte(`{"a":2}`)
	if sign.Verify([]string{pub}, res) {
		t.Error("변조된 결과가 검증 통과 — 무결성 실패")
	}

	// 다른 키 → 실패.
	res.CbomCyclonedx = []byte(`{"a":1}`)
	otherPub, _, _ := sign.Generate()
	if sign.Verify([]string{otherPub}, res) {
		t.Error("다른 공개키로 검증 통과")
	}

	// 서명 없음 → 실패.
	res.Envelope.Signature = ""
	if sign.Verify([]string{pub}, res) {
		t.Error("무서명이 검증 통과")
	}
}
