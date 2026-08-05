package ingest

import (
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/discovery/normalize"
	"github.com/pqcota/pqcota/pkg/kernel/scope"
)

// IngestReport — 중앙 적재 결과 요약.
type IngestReport struct {
	Accepted  int // 수용된 CollectionResult 수
	OffScope  int // 미등재/앵커 없음 → 등재 판정 요청(§0.4)
	Rejected  int // 서명 검증 실패 → 거부(§2.7)
	Snapshots int // 적재된 노드 수(= 관측 기록 건수)
	Changed   int // 그중 실질 내용이 바뀌어 **새 스냅샷**이 생긴 노드 수
	// ExcludedByScope — 자산 스코프 정책으로 관리 대상에서 뺀 finding 수(제외 ≠ 부재 — 고지용).
	ExcludedByScope int
	Nodes           []string           // 저장된 노드
	Notes           []string           // off-scope/거부 사유
	Conflicts       []IdentityConflict // node_id↔지문 중복/충돌(사용자 입력 검증, §0.4)
}

// IngestResults — 회수된 CollectionResult[]를 중앙 인벤토리(히스토리)에 적재한다.
// 엣지↔중앙 경계를 넘어온 계약을 받아 노드별로 정규화·영속화하는 관문이다.
//
// 흐름(§0.4·§2.7·§2.5): 노드별 그룹핑 → 스코프 게이트 → 서명검증 → Normalize → history append.
//   - master=nil  : 스코프 게이트 생략(로컬/데모 — CMDB 없음).
//   - verifySig=nil: 서명검증 생략(전송 보안이 대신할 때, 예 mTLS/SSH).
//
// 미등재 노드는 저장하지 않고 등재 판정 요청으로 센다(§0.4 — 임의 수집 금지).
func IngestResults(
	results []*discoveryv1.CollectionResult,
	master *scope.Master,
	verifySig func(*discoveryv1.CollectionResult) bool,
	snapshotPrefix, rulesetVersion string,
	store history.Store,
	assetPolicy *scope.AssetPolicy, // nil이면 관측된 자산 전부가 관리 대상
) (*IngestReport, error) {
	rep := &IngestReport{}
	rep.Conflicts = CheckIdentity(results) // 사용자 node_id 입력의 중복/충돌을 지문으로 교차검증(§0.4)
	byNode := map[string][]*discoveryv1.CollectionResult{}
	var order []string

	for _, res := range results {
		node := res.GetEnvelope().GetTargetNodeId()
		if node == "" {
			rep.OffScope++
			rep.Notes = append(rep.Notes, "타깃 노드 미지정 — 스코프 앵커 없음")
			continue
		}
		if master != nil && !master.Registered(node) {
			rep.OffScope++
			rep.Notes = append(rep.Notes, node+": 미등재 → 등재 판정 요청(§0.4)")
			continue
		}
		if verifySig != nil && !verifySig(res) {
			rep.Rejected++
			rep.Notes = append(rep.Notes, node+": 서명 검증 실패 → 거부(§2.7)")
			continue
		}
		if _, ok := byNode[node]; !ok {
			order = append(order, node)
		}
		byNode[node] = append(byNode[node], res)
		rep.Accepted++
	}

	// 노드별로 정규화 → append-only 히스토리 적재(§2.5⑥). 결정론: 같은 입력·규칙 → 같은 finding id.
	for _, node := range order {
		snapID := snapshotPrefix + ":" + node
		snap, err := normalize.Normalize(byNode[node], snapID, node, rulesetVersion, store, assetPolicy)
		if err != nil {
			return rep, err
		}
		rep.Snapshots++
		rep.ExcludedByScope += snap.ExcludedByScope
		if snap.Created { // 내용이 직전과 같으면 스냅샷은 새로 안 생기고 관측 기록만 남는다
			rep.Changed++
		}
		rep.Nodes = append(rep.Nodes, node)
	}
	return rep, nil
}

// IngestCBOM — 외부 도구(CBOMkit/CipherIQ 등)가 낸 CycloneDX를 수신해 히스토리에 적재한다(SV-2·SD-7).
// ImportCBOM(서명검증→구조검증→스코프바인딩, §2.3)을 통과한 것만 CollectionResult로 감싸
// Normalize→store 한다. pqcota collector 경로(IngestResults)와 같은 뒷단을 공유 — 입력 형식만 다름.
func IngestCBOM(raw []byte, targetNodeID string, verifySig func([]byte) bool, snapshotPrefix, rulesetVersion string, store history.Store) (Disposition, error) {
	disp, env, _ := ImportCBOM(raw, targetNodeID, verifySig)
	if disp != Accepted {
		return disp, nil // Rejected(검증실패)·NeedsScopeBinding(앵커없음)은 저장하지 않는다.
	}
	res := &discoveryv1.CollectionResult{
		Envelope:             env, // ARTIFACT Envelope(SV-2)
		CbomCyclonedx:        raw,
		CyclonedxSpecVersion: "1.6",
		// 받은 바이트를 그대로 원본으로 남긴다 — 본문은 내부 정규 버전으로 수렴할 수 있어
		// 나중에 "무엇을 받았나"를 되짚으려면 수렴 전 바이트가 필요하다(§2.5 step 1).
		RawCapture: raw,
		RawFormat:  "external-cbom/cyclonedx",
	}
	snapID := snapshotPrefix + ":" + targetNodeID
	if _, err := normalize.Normalize([]*discoveryv1.CollectionResult{res}, snapID, targetNodeID, rulesetVersion, store, nil); err != nil {
		return disp, err
	}
	return Accepted, nil
}
