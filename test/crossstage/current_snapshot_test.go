// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

// 노드의 현재 스냅샷과 생성 기록(pqcota-inventory#1). 같은 노드에 A → B → A를 적재한 뒤 생성 기록을 만든다.
// pqcota-provision은 조치의 finding을 노드의 Latest에서 찾아 before와 앱 키로 쓴다. 세 번째 적재가 첫 A를
// 다시 써도(스냅샷은 2개), before와 앱 키는 B가 아니라 마지막에 적재한 A의 것이어야 한다.
// A와 B는 libssl.so.3의 판과 앱 키가 다르다 — before(라이브러리@판)와 앱 키 둘 다로 가른다.

import (
	"testing"

	commonv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/common/v1"
	discoveryv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/discovery/v1"
	provisioningv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/provisioning/v1"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/history"
	"github.com/randyinthedev-hash/pqcota-provisioning/pkg/provisioning"
)

func libsslSnapshot(id, version string, appKeys ...string) *history.Snapshot {
	return &history.Snapshot{ID: id, NodeID: "appkey-node", RulesetVersion: "r1", Findings: []*discoveryv1.Finding{{
		Id:            "libssl",
		CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		RuntimeAxes:   &discoveryv1.Finding_Openssl{Openssl: &discoveryv1.OpensslAxes{Lib: "libssl.so.3", Version: version}},
		AppKeys:       appKeys,
	}}}
}

func TestProvisioningRecordUsesTheLastIngestedState(t *testing.T) {
	store := history.NewMemStore()
	for _, s := range []*history.Snapshot{
		libsslSnapshot("A", "3.0.1", "user@1001.service/svc-a.service", "user@1001.service/svc-b.service"),
		libsslSnapshot("B", "3.0.2", "user@1001.service"),
		libsslSnapshot("A2", "3.0.1", "user@1001.service/svc-a.service", "user@1001.service/svc-b.service"),
	} {
		if err := store.Append(s); err != nil {
			t.Fatal(err)
		}
	}
	if snaps, _ := store.Snapshots("appkey-node"); len(snaps) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(snaps))
	}

	// pqcota-provision과 같은 순서: 노드의 Latest에서 조치의 finding을 찾고 그것으로 기록을 만든다.
	latest, err := store.Latest("appkey-node")
	if err != nil || latest == nil {
		t.Fatalf("Latest: %v %v", latest, err)
	}
	var f *discoveryv1.Finding
	for _, x := range latest.Findings {
		if x.GetId() == "libssl" {
			f = x
		}
	}
	if f == nil {
		t.Fatal("the action's finding is not in the latest snapshot")
	}
	action := &provisioningv1.RemediationAction{Id: "a1", FindingId: "libssl", TargetNodeId: "appkey-node"}
	rec := provisioning.NewProvisioningRecord("aba:a1", "appkey-node", f.GetAppKeys(), "aba", action, []*discoveryv1.Finding{f})

	if got := rec.GetBefore().GetModules(); len(got) != 1 || got[0] != "libssl.so.3@3.0.1" {
		t.Errorf("before = %v, want [libssl.so.3@3.0.1] (A) — B would be 3.0.2", got)
	}
	if got := rec.GetAppKeys(); len(got) != 2 || got[0] != "user@1001.service/svc-a.service" || got[1] != "user@1001.service/svc-b.service" {
		t.Errorf("app keys = %v, want the two services of A", got)
	}
}
