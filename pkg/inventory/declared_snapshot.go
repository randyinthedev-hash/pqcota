package inventory

import (
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/inventory/declaration"
)

// IsDeclarationOnly — 이 스냅샷이 **선언만** 담고 있나.
//
// 선언은 노드의 상태가 아니다. 사람이 "이 엣지는 그 앱 것이다"라고 적은 것이지 그 노드를 다시
// 관측한 결과가 아니므로, **그것이 노드의 "현재"가 되면 안 된다.** 그러면 직전에 관측한 자산과
// 엣지가 화면에서 사라지고, 읽는 사람은 없어진 줄 안다 — 이 리포가 가장 피하는 모양이다.
//
// 판정은 내용으로 한다: 자산이 하나도 없고, 엣지가 전부 선언 표시를 달고 있으면 선언이다.
// 관측 결과는 자산이든 엣지든 관측된 것을 담고 있으므로 여기 걸리지 않는다.
func IsDeclarationOnly(s *history.Snapshot) bool {
	if s == nil || len(s.Findings) > 0 || len(s.Edges) == 0 {
		return false
	}
	for _, e := range s.Edges {
		if e.GetAppKeyKind() != declaration.KindDeclared {
			return false
		}
	}
	return true
}

// LatestObserved — 그 노드에서 **관측된** 최신 스냅샷. 선언만 담은 것은 건너뛴다.
//
// [history.Store.Latest]를 바꾸지 않는 이유: 절단(보존 정책)의 "최신 불가침"과 프로비저닝의
// before 캡처가 그것을 쓴다. 그쪽은 "가장 최근에 쌓인 것"이 맞고, 여기는 "가장 최근에 본 것"이
// 맞다 — 둘이 다르므로 함수도 다르다.
func LatestObserved(store history.Store, nodeID string) (*history.Snapshot, error) {
	latest, err := store.Latest(nodeID)
	if err != nil || !IsDeclarationOnly(latest) {
		return latest, err
	}
	snaps, err := store.Snapshots(nodeID) // seq 오름차순
	if err != nil {
		return nil, err
	}
	for i := len(snaps) - 1; i >= 0; i-- {
		if !IsDeclarationOnly(snaps[i]) {
			return snaps[i], nil
		}
	}
	return nil, nil // 관측이 하나도 없다 — 선언만 있는 노드다
}
