package normalize_test

import (
	"testing"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/discovery/normalize"
)

// openssl-collector가 낼 법한 CollectionResult(CycloneDX + Envelope + 완전성) 픽스처.
func opensslResult(node string) *discoveryv1.CollectionResult {
	cbom := []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","components":[
      {"type":"cryptographic-asset","name":"libcrypto","properties":[
        {"name":"pqcota:crypto_runtime","value":"openssl"},
        {"name":"pqcota:detection_method","value":"runtime-introspection+symbol-analysis"},
        {"name":"pqcota:openssl.fork","value":"OpenSSL"},
        {"name":"pqcota:openssl.binding_mode","value":"dynamic"}]}]}`)
	return &discoveryv1.CollectionResult{
		Envelope: &commonv1.Envelope{
			TargetNodeId:    node,
			DetectionMethod: commonv1.DetectionMethod_DETECTION_METHOD_RUNTIME_INTROSPECTION,
		},
		CbomCyclonedx:        cbom,
		CyclonedxSpecVersion: "1.6",
		Completeness: &commonv1.Completeness{
			LayersCovered: []commonv1.CollectionLayer{commonv1.CollectionLayer_COLLECTION_LAYER_PROCESS},
		},
	}
}

func TestDeriveFindings(t *testing.T) {
	fs, err := normalize.DeriveFindings(opensslResult("cmdb://n1"), "snap-1", "ruleset-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1", len(fs))
	}
	f := fs[0]
	if f.GetCryptoRuntime() != commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL {
		t.Errorf("crypto_runtime = %v, want OPENSSL", f.GetCryptoRuntime())
	}
	if f.GetEvidenceStrength() != commonv1.EvidenceStrength_EVIDENCE_STRENGTH_CONFIRMED {
		t.Errorf("evidence_strength = %v, want CONFIRMED (runtime-introspection)", f.GetEvidenceStrength())
	}
	if f.GetOpenssl().GetFork() != "OpenSSL" {
		t.Errorf("fork = %q", f.GetOpenssl().GetFork())
	}
	if f.GetOpenssl().GetBindingMode() != commonv1.OpensslBindingMode_OPENSSL_BINDING_MODE_DYNAMIC {
		t.Errorf("binding_mode = %v, want DYNAMIC", f.GetOpenssl().GetBindingMode())
	}
	if f.GetDerivedFromSnapshotId() != "snap-1" || f.GetRulesetVersion() != "ruleset-1" {
		t.Errorf("파생 추적 필드 누락: %+v", f)
	}
	if f.GetId() == "" {
		t.Error("finding id(정규화 해시) 비어 있음")
	}
}

// #3 다중 앱 귀속: pqcota:app_keys CSV → Finding.app_keys(정렬·복수).
func TestDeriveFindings_MultiApp(t *testing.T) {
	cbom := []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","components":[
      {"type":"cryptographic-asset","name":"libcrypto","properties":[
        {"name":"pqcota:crypto_runtime","value":"openssl"},
        {"name":"pqcota:app_keys","value":"api.service,payment.service"}]}]}`)
	res := &discoveryv1.CollectionResult{
		Envelope:      &commonv1.Envelope{TargetNodeId: "cmdb://n1"},
		CbomCyclonedx: cbom,
	}
	fs, err := normalize.DeriveFindings(res, "s", "r")
	if err != nil {
		t.Fatal(err)
	}
	if got := fs[0].GetAppKeys(); len(got) != 2 || got[0] != "api.service" || got[1] != "payment.service" {
		t.Errorf("공유 .so 다중 귀속 파싱 실패: %v", got)
	}
}

func jcaResult(node, providerSet string) *discoveryv1.CollectionResult {
	cbom := []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","components":[
      {"type":"cryptographic-asset","name":"jca-provider-chain","properties":[
        {"name":"pqcota:crypto_runtime","value":"jca"},
        {"name":"pqcota:detection_method","value":"runtime-introspection"},
        {"name":"pqcota:jca.provider_set","value":"` + providerSet + `"}]}]}`)
	return &discoveryv1.CollectionResult{
		Envelope:      &commonv1.Envelope{TargetNodeId: node},
		CbomCyclonedx: cbom,
	}
}

func TestDeriveFindings_JCA(t *testing.T) {
	// BC 포함 → 전 표준 알고리즘 커버, SLH-DSA 갭 없음.
	fs, _ := normalize.DeriveFindings(jcaResult("cmdb://j1", "SUN,BC"), "s", "r")
	f := fs[0]
	if f.GetCryptoRuntime() != commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA {
		t.Fatalf("crypto_runtime = %v, want JCA", f.GetCryptoRuntime())
	}
	if len(f.GetJca().GetProviderSet()) != 2 {
		t.Errorf("provider_set = %v", f.GetJca().GetProviderSet())
	}
	if f.GetPqcReadiness() != "provider-보강(전 표준 알고리즘)" {
		t.Errorf("pqc_readiness = %q (BC는 SLH-DSA 커버)", f.GetPqcReadiness())
	}

	// JDK 네이티브만(SunJCE) → SLH-DSA 갭.
	fs2, _ := normalize.DeriveFindings(jcaResult("cmdb://j2", "SUN,SunJCE"), "s", "r")
	if got := fs2[0].GetPqcReadiness(); got != "provider-보강(SLH-DSA 갭)" {
		t.Errorf("pqc_readiness = %q, want SLH-DSA 갭 (§2.3)", got)
	}
}

func TestNormalizePipeline(t *testing.T) {
	store := history.NewMemStore()

	// dedup: 동일 노드·컴포넌트를 두 번 넣어도 하나로 수렴(§2.5⑤).
	results := []*discoveryv1.CollectionResult{opensslResult("cmdb://n1"), opensslResult("cmdb://n1")}
	snap, err := normalize.Normalize(results, "snap-1", "cmdb://n1", "ruleset-1", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Findings) != 1 {
		t.Fatalf("dedup 실패: %d findings, want 1", len(snap.Findings))
	}
	// 히스토리 append 확인.
	if latest, _ := store.Latest("cmdb://n1"); latest == nil || latest.ID != "snap-1" {
		t.Error("히스토리에 스냅샷 append 안 됨")
	}

	// 재현성: 같은 입력+ruleset → 같은 finding id(§0.2).
	snap2, _ := normalize.Normalize([]*discoveryv1.CollectionResult{opensslResult("cmdb://n1")}, "snap-2", "cmdb://n1", "ruleset-1", store, nil)
	if snap.Findings[0].GetId() != snap2.Findings[0].GetId() {
		t.Error("결정론 위반: 같은 입력인데 finding id 다름")
	}
	// 같은 내용을 다시 관측한 것이라 스냅샷은 늘지 않고(중복 억제) 관측 기록만 쌓인다.
	if snaps, _ := store.Snapshots("cmdb://n1"); len(snaps) != 1 {
		t.Errorf("같은 내용 재관측 → 스냅샷 %d건, want 1", len(snaps))
	}
	if stats, _ := store.ObservationStats("cmdb://n1"); stats["snap-1"].Count != 2 {
		t.Errorf("관측 횟수 = %d, want 2 (봤다는 사실은 보존)", stats["snap-1"].Count)
	}
}
