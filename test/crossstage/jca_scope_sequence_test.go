// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

// 같은 배포본을 전체 스캔 → --pid 스캔 → 전체 스캔 순서로 적재한다(v0.12.0 태그 전 검증, 계획 §8).
//   - finding id는 세 번 모두 같다(관측 범위는 id에 들어가지 않는다).
//   - 두 전환은 관측 범위가 바뀐 것이라 diff의 changed에 나온다.
//   - 가운데 스냅샷으로는 before를 채우지 않고 사유가 남는다.
//   - 마지막 스냅샷은 두 JVM의 체인을 모두 봤고 같으므로 before가 다시 채워진다.

import (
	"strings"
	"testing"

	discoveryv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota-discovery/collectors/jvm"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/history"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/normalize"
	"github.com/randyinthedev-hash/pqcota-provisioning/pkg/provisioning"
)

func TestFullThenPIDThenFullScan(t *testing.T) {
	svc := "0::/system.slice/pay.service\n"
	j1 := jvm.JVMProc{PID: 101, JavaHome: "/opt/jdk", Cgroup: svc, Cmdline: "/opt/jdk/bin/java\x00-jar\x00/opt/pay/app.jar\x00"}
	j2 := jvm.JVMProc{PID: 102, JavaHome: "/opt/jdk", Cgroup: svc, Cmdline: "/opt/jdk/bin/java\x00-jar\x00/opt/pay/app.jar\x00"}
	chain := func() *jvm.Collected { c := jvm.ParseProviders(providers); return &c }

	full := func() *discoveryv1.CollectionResult {
		return jvm.BuildDeploymentResult("pay", jvm.GroupDeployments([]jvm.Member{{JVM: j1, Collected: chain()}, {JVM: j2, Collected: chain()}})[0])
	}
	pid := jvm.BuildDeploymentResult("pay", jvm.SelectPID(jvm.GroupDeployments([]jvm.Member{{JVM: j1, Collected: chain()}})[0], []jvm.JVMProc{j1, j2}, 101))

	store := history.NewMemStore()
	var snaps []*history.Snapshot
	for i, r := range []*discoveryv1.CollectionResult{full(), pid, full()} {
		s, err := normalize.Normalize([]*discoveryv1.CollectionResult{r}, "snap-"+string(rune('a'+i)), "pay", normalize.RulesetVersion, store, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Findings) != 1 {
			t.Fatalf("snapshot %d: %d findings", i, len(s.Findings))
		}
		snaps = append(snaps, s)
	}
	id := snaps[0].Findings[0].GetId()
	for i, s := range snaps {
		if s.Findings[0].GetId() != id {
			t.Errorf("snapshot %d: finding id %s, want %s", i, s.Findings[0].GetId(), id)
		}
	}
	for i := 1; i < len(snaps); i++ {
		if history.ContentHashV1(snaps[i-1]) == history.ContentHashV1(snaps[i]) {
			t.Errorf("transition %d→%d: the fingerprint did not change", i-1, i)
		}
		d := inventory.RenderDiff(snaps[i-1], snaps[i])
		if !strings.Contains(d, "~ "+id[:12]) || strings.Contains(d, "+ ") || strings.Contains(d, "- ") {
			t.Errorf("transition %d→%d: not shown as one changed deployment:\n%s", i-1, i, d)
		}
		if !strings.Contains(d, "scope=selected-pid") {
			t.Errorf("transition %d→%d: the diff does not show the scope change:\n%s", i-1, i, d)
		}
	}
	for i, want := range []string{"SUN,SunJCE", "", "SUN,SunJCE"} {
		st := provisioning.CaptureState(snaps[i].Findings)
		why := provisioning.BeforeGap(snaps[i].Findings)
		if got := strings.Join(st.GetProviderChain(), ","); got != want {
			t.Errorf("snapshot %d: before chain %q, want %q", i, got, want)
		}
		if (want == "") != (why != "") {
			t.Errorf("snapshot %d: before reason %q", i, why)
		}
	}
	if why := provisioning.BeforeGap(snaps[1].Findings); !strings.Contains(why, "selected with --pid (2 JVM(s) found") {
		t.Errorf("middle snapshot reason = %q", why)
	}
}
