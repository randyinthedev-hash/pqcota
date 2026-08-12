package inventory

import (
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/inventory/declaration"
)

// AttributionOverlay — 선언된 엣지→앱 귀속을 조회 시점에 얹기 위한 색인.
//
// **저장을 고치지 않는다.** 선언은 자기 레인에 그대로 있고, 관측 엣지도 그대로다. 둘을 합치는
// 일은 여기서, **읽을 때만** 일어난다 — 적재가 관측을 고치면 collector의 서명과 어긋나고,
// `raw_capture`에서 다시 계산할 때 저장된 값과 갈린다(검토 중인 설계 §5.2).
type AttributionOverlay struct {
	byEdge map[edgeKey]string
}

type edgeKey struct {
	node string
	dst  string
	port uint32
}

// BuildAttributionOverlay — 선언 레인 스냅샷들에서 색인을 만든다.
//
// 관측 스냅샷을 넣어도 안전하다 — 선언 레인 표시(`app_key_kind="declared"`)가 없는 엣지는
// 무시하므로, 관측이 이 색인에 섞여 들어오지 않는다.
func BuildAttributionOverlay(snaps ...*history.Snapshot) *AttributionOverlay {
	o := &AttributionOverlay{byEdge: map[edgeKey]string{}}
	for _, s := range snaps {
		if s == nil {
			continue
		}
		for _, e := range s.Edges {
			if e.GetAppKeyKind() != declaration.KindDeclared || e.GetAppKey() == "" {
				continue
			}
			o.byEdge[edgeKey{e.GetSrcNodeId(), e.GetDstAddr(), e.GetPort()}] = e.GetAppKey()
		}
	}
	return o
}

// Apply — 엣지 하나에 선언을 얹는다. 관측이 이미 채운 자리는 **손대지 않는다.**
//
// 덮어쓰게 두면 사람이 적은 것과 기계가 본 것이 섞이고, 선언 레인을 따로 둔 이유가 사라진다.
// 원본 엣지도 고치지 않는다 — 화면에 낼 값만 돌려준다.
func (o *AttributionOverlay) Apply(e *discoveryv1.ObservedEdge) (key, kind string) {
	if e.GetAppKey() != "" {
		return e.GetAppKey(), e.GetAppKeyKind() // 관측이 이겼다
	}
	if o == nil {
		return "", ""
	}
	if k, ok := o.byEdge[edgeKey{e.GetSrcNodeId(), e.GetDstAddr(), e.GetPort()}]; ok {
		return k, declaration.KindDeclared
	}
	return "", ""
}

// Len — 색인에 든 선언 수.
func (o *AttributionOverlay) Len() int {
	if o == nil {
		return 0
	}
	return len(o.byEdge)
}
