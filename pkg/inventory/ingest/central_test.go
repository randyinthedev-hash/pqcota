package ingest_test

import (
	"strings"
	"testing"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/discovery/history"
	"github.com/randyinthedev-hash/pqcota/pkg/inventory"
	"github.com/randyinthedev-hash/pqcota/pkg/inventory/ingest"
	"github.com/randyinthedev-hash/pqcota/pkg/kernel/scope"
	"github.com/randyinthedev-hash/pqcota/pkg/kernel/sign"
)

func edgeResult(node string) *discoveryv1.CollectionResult {
	return &discoveryv1.CollectionResult{
		Envelope: &commonv1.Envelope{TargetNodeId: node},
		ObservedEdges: []*discoveryv1.ObservedEdge{{
			SrcNodeId:       node,
			DstAddr:         "10.0.0.9:443",
			Protocol:        discoveryv1.NetworkProtocol_NETWORK_PROTOCOL_TLS,
			NegotiatedGroup: "X25519MLKEM768",
		}},
	}
}

// 스코프 게이트(§1.4) + Normalize + history 적재 + 엣지 부착을 종단 검증.
func TestIngestResults(t *testing.T) {
	master := scope.NewMaster([]string{"web-01"})
	store := history.NewMemStore()
	results := []*discoveryv1.CollectionResult{
		edgeResult("web-01"),
		edgeResult("rogue"),                              // 미등재 → 등재요청
		{Envelope: &commonv1.Envelope{TargetNodeId: ""}}, // 앵커 없음 → off-scope
	}
	rep, err := ingest.IngestResults(results, master, nil, "snap", "ruleset-1", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Accepted != 1 || rep.OffScope != 2 || rep.Snapshots != 1 {
		t.Fatalf("report = %+v (want accepted=1 offscope=2 snapshots=1)", rep)
	}
	snap, _ := store.Latest("web-01")
	if snap == nil || len(snap.Edges) != 1 {
		t.Fatalf("web-01 스냅샷에 엣지 미저장: %+v", snap)
	}
	if s, _ := store.Latest("rogue"); s != nil {
		t.Error("미등재 노드가 적재됨 — 스코프 게이트 실패")
	}
}

// 서명 검증 실패는 거부(§2.6).
func TestIngestSignatureReject(t *testing.T) {
	master := scope.NewMaster([]string{"web-01"})
	store := history.NewMemStore()
	rep, _ := ingest.IngestResults([]*discoveryv1.CollectionResult{edgeResult("web-01")},
		master, func(*discoveryv1.CollectionResult) bool { return false }, "snap", "r", store, nil)
	if rep.Rejected != 1 || rep.Accepted != 0 || rep.Snapshots != 0 {
		t.Errorf("서명 거부 실패: %+v", rep)
	}
}

// master=nil이면 스코프 게이트 생략(로컬/데모) — 전부 수용.
func TestIngestNoMaster(t *testing.T) {
	store := history.NewMemStore()
	rep, _ := ingest.IngestResults([]*discoveryv1.CollectionResult{edgeResult("a"), edgeResult("b")},
		nil, nil, "snap", "r", store, nil)
	if rep.Accepted != 2 || rep.Snapshots != 2 {
		t.Errorf("no-master 적재 실패: %+v", rep)
	}
}

// 자산 하나가 담긴 CycloneDX — 스코프 정책이 걸 대상이 있어야 제외를 셀 수 있다.
func opensslResult(node, lib string) *discoveryv1.CollectionResult {
	cbom := []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","components":[
      {"type":"cryptographic-asset","name":"` + lib + `","properties":[
        {"name":"pqcota:crypto_runtime","value":"openssl"},
        {"name":"pqcota:detection_method","value":"runtime-introspection"},
        {"name":"pqcota:app_keys","value":"/opt/apps/internal-test"}]}]}`)
	return &discoveryv1.CollectionResult{
		Envelope:             &commonv1.Envelope{TargetNodeId: node},
		CbomCyclonedx:        cbom,
		CyclonedxSpecVersion: "1.6",
	}
}

// TV-SCOPE-7 — 제외는 "없음"이 아니다(§2.6·§8.3). 정책으로 뺀 건수가 적재 요약과 인벤토리
// 뷰 **양쪽**에 남아야 한다. 스코프가 조용히 자산을 지우면 인벤토리가 거짓말을 한다.
func TestIngestReportsScopeExclusions(t *testing.T) {
	policy, err := scope.LoadAssetPolicy(strings.NewReader(
		"exclude,openssl,libtest.so.*,/opt/apps/internal-test,테스트 앱 제외\n"))
	if err != nil {
		t.Fatal(err)
	}
	store := history.NewMemStore()
	rep, err := ingest.IngestResults(
		[]*discoveryv1.CollectionResult{opensslResult("web-01", "libtest.so.1")},
		nil, nil, "snap", "r", store, policy)
	if err != nil {
		t.Fatal(err)
	}
	if rep.ExcludedByScope != 1 {
		t.Fatalf("적재 요약이 제외를 고지하지 않는다: %+v", rep)
	}
	snap, _ := store.Latest("web-01")
	if snap == nil || snap.ExcludedByScope != 1 {
		t.Fatalf("스냅샷에 제외 건수가 남지 않았다: %+v", snap)
	}
	if len(snap.Findings) != 0 {
		t.Errorf("제외한 자산이 그대로 남았다: %d건", len(snap.Findings))
	}
	if out := inventory.RenderDetail(snap); !strings.Contains(out, "excluded by asset scope: 1") {
		t.Errorf("인벤토리 뷰가 제외를 고지하지 않는다:\n%s", out)
	}
}

// TD-SIGN-4 — 거부만 시험하면 게이트가 정상 반입까지 막는 것을 못 잡는다. 서명한 결과가
// 실제로 통과해 적재되는지 본다(에어갭 반입의 정상 경로).
func TestIngestAcceptsValidSignature(t *testing.T) {
	pub, priv, err := sign.Generate()
	if err != nil {
		t.Fatal(err)
	}
	res := edgeResult("web-01")
	sig, err := sign.Sign(priv, res)
	if err != nil {
		t.Fatal(err)
	}
	res.Envelope.Signature = sig
	store := history.NewMemStore()
	rep, err := ingest.IngestResults([]*discoveryv1.CollectionResult{res},
		scope.NewMaster([]string{"web-01"}),
		func(r *discoveryv1.CollectionResult) bool { return sign.Verify([]string{pub}, r) },
		"snap", "r", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Rejected != 0 || rep.Accepted != 1 || rep.Snapshots != 1 {
		t.Fatalf("정상 서명이 통과하지 못했다: %+v", rep)
	}
}
