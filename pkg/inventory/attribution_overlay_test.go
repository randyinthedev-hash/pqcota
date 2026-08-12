package inventory_test

import (
	"strings"
	"testing"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/inventory"
	"github.com/pqcota/pqcota/pkg/inventory/declaration"
)

func declaredSnap(t *testing.T, csv string) *history.Snapshot {
	t.Helper()
	res, err := declaration.ImportAttributionCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("선언이 하나도 안 만들어졌다")
	}
	return &history.Snapshot{ID: "d1", NodeID: "web-01", Edges: res[0].GetObservedEdges()}
}

func observed(t *testing.T) *history.Snapshot {
	t.Helper()
	return &history.Snapshot{
		ID: "s1", NodeID: "web-01",
		Completeness: &commonv1.Completeness{Note: "엣지 2개 중 1개를 앱에 귀속하지 못했다"},
		Edges: []*discoveryv1.ObservedEdge{
			// 관측이 이미 잡은 것
			{SrcNodeId: "web-01", DstAddr: "10.0.0.5", Port: 443, AppKey: "payment.service", AppKeyKind: "systemd-unit"},
			// 못 잡은 것 — 선언이 메울 자리
			{SrcNodeId: "web-01", DstAddr: "10.0.0.7", Port: 22},
		},
	}
}

// TestDeclarationNeverOverwritesObservation — **이 릴리스에서 가장 중요한 규칙이다.**
//
// 덮어쓰게 두면 사람이 적은 것과 기계가 본 것이 섞이고, 선언 레인을 따로 둔 이유가 사라진다.
func TestDeclarationNeverOverwritesObservation(t *testing.T) {
	// 관측이 이미 채운 자리를 노리는 선언을 일부러 넣는다.
	decl := declaredSnap(t, "node_id,dst,port,app_key\nweb-01,10.0.0.5,443,사람이-적은-다른-앱\nweb-01,10.0.0.7,22,batch-job.service\n")
	out := inventory.RenderDetailWith(observed(t), inventory.BuildAttributionOverlay(decl))

	if strings.Contains(out, "사람이-적은-다른-앱") {
		t.Error("선언이 관측을 덮어썼다")
	}
	if !strings.Contains(out, "@payment.service") {
		t.Error("관측한 귀속이 사라졌다")
	}
	if !strings.Contains(out, "@batch-job.service(declared)") {
		t.Error("빈 자리를 선언으로 메우지 못했다 — 메웠어도 declared 표시가 없다")
	}
	// 화면만 보면 관측과 구별되지 않으므로 몇 개가 선언인지 밝힌다.
	if !strings.Contains(out, "1개는 관측이 아니라") {
		t.Error("몇 개가 선언으로 메워졌는지 안 밝힌다")
	}
}

// TestOverlayDoesNotMutateTheStoredEdge — 저장된 관측은 읽어도 그대로다.
//
// 적재가 관측을 고치면 collector의 서명과 어긋나고, raw_capture에서 다시 계산할 때 저장된 값과
// 갈린다. 그래서 합치는 일은 **읽을 때만** 일어나야 한다.
func TestOverlayDoesNotMutateTheStoredEdge(t *testing.T) {
	snap := observed(t)
	decl := declaredSnap(t, "web-01,10.0.0.7,22,batch-job.service\n")
	_ = inventory.RenderDetailWith(snap, inventory.BuildAttributionOverlay(decl))

	if got := snap.Edges[1].GetAppKey(); got != "" {
		t.Fatalf("저장된 엣지가 %q로 바뀌었다 — 서명이 덮는 필드다", got)
	}
	if got := snap.Edges[1].GetAppKeyKind(); got != "" {
		t.Fatalf("저장된 엣지의 kind가 %q로 바뀌었다", got)
	}
}

// TestObservedEdgesDoNotLeakIntoTheOverlay — 관측 스냅샷을 넣어도 색인이 오염되지 않는다.
//
// 색인은 `app_key_kind="declared"`인 것만 받는다. 안 그러면 관측이 관측을 메우게 되어,
// 어느 것이 근거인지 알 수 없어진다.
func TestObservedEdgesDoNotLeakIntoTheOverlay(t *testing.T) {
	if n := inventory.BuildAttributionOverlay(observed(t)).Len(); n != 0 {
		t.Fatalf("관측 엣지 %d개가 선언 색인에 들어갔다", n)
	}
}

// TestAttributionCSVRefusesWhatItCannotPlace — 어느 엣지를 가리키는지 모르는 줄은 받지 않는다.
func TestAttributionCSVRefusesWhatItCannotPlace(t *testing.T) {
	for _, bad := range []string{
		"web-01,10.0.0.7,포트아님,app\n", // 포트를 못 읽으면 어느 엣지인지 모른다
		"web-01,10.0.0.7,22,\n",      // 귀속할 앱이 없다
		",10.0.0.7,22,app\n",         // 어느 노드인지 모른다
	} {
		if _, err := declaration.ImportAttributionCSV(strings.NewReader(bad)); err == nil {
			t.Errorf("받아들이면 안 되는 줄을 받았다: %q", strings.TrimSpace(bad))
		}
	}
}

// TestDeclarationIsNotTheNodeState — **선언이 노드의 "현재"를 덮으면 안 된다.**
//
// 선언은 사람이 "이 엣지는 그 앱 것이다"라고 적은 것이지 노드를 다시 관측한 결과가 아니다.
// 그것이 최신 스냅샷이 되면 직전에 관측한 자산·엣지가 화면에서 사라지고, 읽는 사람은 없어진
// 줄 안다 — 데모에서 실제로 그렇게 나왔다(관측 엣지 4개가 1개로 보였다).
func TestDeclarationIsNotTheNodeState(t *testing.T) {
	obs := observed(t)
	decl := declaredSnap(t, "web-01,10.0.0.7,22,batch-job.service\n")

	if inventory.IsDeclarationOnly(obs) {
		t.Error("관측 스냅샷을 선언으로 판정했다 — 관측이 화면에서 사라진다")
	}
	if !inventory.IsDeclarationOnly(decl) {
		t.Error("선언 스냅샷을 관측으로 판정했다 — 그것이 노드의 현재가 된다")
	}

	// 선언이 나중에 쌓여도 마지막 **관측**이 나온다.
	store := history.NewMemStore()
	for _, s := range []*history.Snapshot{obs, decl} {
		if err := store.Append(&history.Snapshot{
			ID: s.ID, NodeID: "web-01", RulesetVersion: "r1", Edges: s.Edges, Findings: s.Findings,
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := inventory.LatestObserved(store, "web-01")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.Edges) != 2 {
		t.Fatalf("마지막 관측이 아니라 선언이 나왔다: %+v", got)
	}
}
