package normalize

import (
	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/kernel/scope"
)

// Normalize — 정규화 파이프라인 후단(§2.5 ③~⑥). 한 노드에 대한 CollectionResult들을
// 강화(Finding 파생) → 동일성 해소(dedup) → **자산 스코프 게이트** → 완전성 병합 → 스냅샷 →
// 히스토리 append 한다.
//
// policy가 nil이면 전부 관리 대상이다. 제외분은 버려지되 **세어서 스냅샷에 남긴다** —
// 조용히 사라지면 인벤토리가 "그런 자산은 없다"고 거짓말한다(§2.7 제외 ≠ 부재).
//
// 결정론적: 같은 입력 + 같은 rulesetVersion → 같은 finding id(§0.2 재현성).
func Normalize(results []*discoveryv1.CollectionResult, snapshotID, nodeID, rulesetVersion string,
	store history.Store, policy *scope.AssetPolicy) (*history.Snapshot, error) {
	seen := make(map[string]bool)
	var findings []*discoveryv1.Finding
	var comp *commonv1.Completeness

	seenEdge := make(map[string]bool)
	var edges []*discoveryv1.ObservedEdge

	for _, res := range results {
		fs, err := DeriveFindings(res, snapshotID, rulesetVersion)
		if err != nil {
			return nil, err
		}
		for _, f := range fs {
			if seen[f.GetId()] { // 정규화 해시로 dedup(§2.5⑤)
				continue
			}
			seen[f.GetId()] = true
			findings = append(findings, f)
		}
		// 관측 통신 엣지(network-collector, §12)도 스냅샷 관측 레인에 싣는다. 노드 내부 자산과 별도.
		for _, e := range res.GetObservedEdges() {
			k := edgeKey(e)
			if seenEdge[k] {
				continue
			}
			seenEdge[k] = true
			edges = append(edges, e)
		}
		comp = mergeCompleteness(comp, res.GetCompleteness())
	}

	// 자산 스코프 게이트 — 노드 게이트(§0.4)를 자산 단위로 넓힌 것. 사용자가 선언한
	// 관리 대상만 남기고, 뺀 수는 고지용으로 보존한다.
	kept, excluded := policy.Apply(findings)

	snap := &history.Snapshot{
		ID:              snapshotID,
		NodeID:          nodeID,
		Findings:        kept,
		Edges:           edges,
		Completeness:    comp,
		RulesetVersion:  rulesetVersion,
		ExcludedByScope: excluded,
	}
	if store != nil {
		if err := store.Append(snap); err != nil { // append-only(§0.2)
			return nil, err
		}
	}
	return snap, nil
}

// edgeKey — 엣지 동일성(§6.2 dedup). 방향+프로토콜+협상그룹 기준.
func edgeKey(e *discoveryv1.ObservedEdge) string {
	dst := e.GetDstNodeId()
	if dst == "" {
		dst = e.GetDstAddr()
	}
	return e.GetSrcNodeId() + "|" + dst + "|" + e.GetProtocol().String() + "|" + e.GetNegotiatedGroup()
}

// mergeCompleteness — 여러 collector의 완전성을 합친다. covered는 합집합, missing은
// (a.missing ∪ b.missing) \ covered. 한 collector가 못 본 계층을 다른 collector가 커버하면 갭 아님.
func mergeCompleteness(a, b *commonv1.Completeness) *commonv1.Completeness {
	if a == nil {
		a = &commonv1.Completeness{}
	}
	if b == nil {
		return a
	}
	coveredSet := map[commonv1.CollectionLayer]bool{}
	for _, l := range a.GetLayersCovered() {
		coveredSet[l] = true
	}
	for _, l := range b.GetLayersCovered() {
		coveredSet[l] = true
	}
	missingSet := map[commonv1.CollectionLayer]bool{}
	for _, l := range append(a.GetLayersMissing(), b.GetLayersMissing()...) {
		if !coveredSet[l] {
			missingSet[l] = true
		}
	}
	note := a.GetNote()
	if note == "" {
		note = b.GetNote()
	}
	return &commonv1.Completeness{
		LayersCovered: keys(coveredSet),
		LayersMissing: keys(missingSet),
		Note:          note,
	}
}

func keys(m map[commonv1.CollectionLayer]bool) []commonv1.CollectionLayer {
	out := make([]commonv1.CollectionLayer, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
