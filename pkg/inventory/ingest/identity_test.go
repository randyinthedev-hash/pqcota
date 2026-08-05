package ingest

import (
	"testing"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
)

func res(nodeID, machineKey string) *discoveryv1.CollectionResult {
	return &discoveryv1.CollectionResult{Envelope: &commonv1.Envelope{
		TargetNodeId: nodeID,
		Machine:      &commonv1.MachineIdentity{SelfAssignedId: machineKey},
	}}
}

// 사용자 node_id 입력의 중복(한 머신 여러 이름)·충돌(한 이름 여러 머신)을 지문으로 잡는다.
func TestCheckIdentity(t *testing.T) {
	results := []*discoveryv1.CollectionResult{
		res("web-01", "node:aaa"),
		res("web-dup", "node:aaa"), // 같은 물리머신(node:aaa)을 다른 이름으로 → 중복
		res("db-01", "node:bbb"),
		res("db-01", "node:ccc"), // 같은 이름을 다른 머신에 → 충돌
		res("clean", "node:ddd"), // 정상
	}
	conflicts := CheckIdentity(results)

	var dup, col bool
	for _, c := range conflicts {
		if c.Kind == "duplicate" && c.Key == "node:aaa" {
			dup = true
			if len(c.Members) != 2 {
				t.Errorf("중복 멤버 수: %v", c.Members)
			}
		}
		if c.Kind == "collision" && c.Key == "db-01" {
			col = true
		}
	}
	if !dup {
		t.Error("중복(한 머신 여러 node_id) 미탐지")
	}
	if !col {
		t.Error("충돌(한 node_id 여러 머신) 미탐지")
	}
	// 정상 항목은 conflict 아님
	for _, c := range conflicts {
		if c.Key == "node:ddd" || c.Key == "clean" {
			t.Errorf("정상 항목이 conflict로: %+v", c)
		}
	}
}
