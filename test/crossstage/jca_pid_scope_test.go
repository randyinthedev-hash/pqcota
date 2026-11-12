// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

// `pqcota-jvmscan --pid`의 결과를 정규화(inventory)와 생성 기록(provisioning)에 잇는다(v0.12.0).
//
// --pid는 지정한 JVM 하나만 관측한다. 그 결과가 전체 스캔과 같은 배포본 키를 가지면서 다른 JVM을 빼 버리면, JVM이
// 여럿인 서비스가 count=1로 보이고 그 한 JVM의 체인이 배포본 전체의 before로 기록될 수 있다. 그래서 수집기는 관측
// 범위(selected-pid)를 표시하고 같은 배포본의 다른 JVM을 체인 없는 변형으로 남기며, 생성은 계속하되 before를 비우고
// 그 사유를 기록에 남긴다.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	discoveryv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota-discovery/collectors/jvm"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/history"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/normalize"
	"github.com/randyinthedev-hash/pqcota-provisioning/pkg/provisioning"
)

// 같은 서비스의 JVM 둘 가운데 하나만 --pid로 관측한 결과.
func pidScopedSnapshot(t *testing.T, node string) *history.Snapshot {
	t.Helper()
	svc := "0::/system.slice/pay.service\n"
	j1 := jvm.JVMProc{PID: 101, JavaHome: "/opt/jdk", Cgroup: svc, Cmdline: "/opt/jdk/bin/java\x00-jar\x00/opt/pay/app.jar\x00"}
	j2 := jvm.JVMProc{PID: 102, JavaHome: "/opt/jdk", Cgroup: svc, Cmdline: "/opt/jdk/bin/java\x00-jar\x00/opt/pay/app.jar\x00"}
	c := jvm.ParseProviders(providers)
	d := jvm.SelectPID(jvm.GroupDeployments([]jvm.Member{{JVM: j1, Collected: &c}})[0], []jvm.JVMProc{j1, j2}, 101)
	r := jvm.BuildDeploymentResult(node, d)
	snap, err := normalize.Normalize([]*discoveryv1.CollectionResult{r}, "snap-"+node, node, normalize.RulesetVersion, history.NewMemStore(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestPIDScopedResultDoesNotStandForTheDeployment(t *testing.T) {
	snap := pidScopedSnapshot(t, "pay")
	if len(snap.Findings) != 1 {
		t.Fatalf("%d findings, want 1", len(snap.Findings))
	}
	f := snap.Findings[0]
	j := f.GetJca()
	if j.GetObservationScope() != jvm.ScopeSelectedPID || j.GetAppKeyKind() != jvm.KindSystemdUnit {
		t.Fatalf("scope %q kind %q", j.GetObservationScope(), j.GetAppKeyKind())
	}
	var jvms, unobserved uint32
	for _, in := range j.GetInstances() {
		jvms += in.GetCount()
		if !in.GetChainObserved() {
			unobserved += in.GetCount()
		}
	}
	if jvms != 2 || unobserved != 1 || len(j.GetProviderSet()) != 0 {
		t.Errorf("instances: %d JVMs, %d unobserved, provider_set %v — want 2, 1, empty", jvms, unobserved, j.GetProviderSet())
	}
	why := provisioning.BeforeGap(snap.Findings)
	if !strings.Contains(why, "selected with --pid") {
		t.Errorf("BeforeGap = %q", why)
	}
	st := provisioning.CaptureState(snap.Findings)
	if len(st.GetProviderChain()) != 0 || len(st.GetModules()) != 0 {
		t.Errorf("the one observed JVM's chain was recorded as the deployment's before: %v", st)
	}
}

// 생성은 계속하되 before를 비운 사유가 실행 기록에 남고, 「before not captured」 집계에 들어간다(실제 Postgres).
func TestPIDScopedResultInTheProvisioningRecord(t *testing.T) {
	dsn := os.Getenv("PQCOTA_TEST_DSN")
	if dsn == "" {
		t.Skip("PQCOTA_TEST_DSN is not set — skipping the Postgres integration test")
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organization, node := "cs-pid-"+suffix, "cs-pid-node-"+suffix
	snap := pidScopedSnapshot(t, node)
	snap.ID = "s-" + suffix
	hist, err := history.NewPgStoreIn(context.Background(), dsn, organization)
	if err != nil {
		t.Fatal(err)
	}
	if err := hist.Append(snap); err != nil {
		t.Fatal(err)
	}
	hist.Close()

	fid := snap.Findings[0].GetId()
	planID := "cs-pid-plan-" + suffix
	dir := t.TempDir()
	plan := filepath.Join(dir, "plan.json")
	writeFile(t, dir, "plan.json", []byte(`{"id":"`+planID+`","status":"PLAN_STATUS_FINALIZED","scope":"ring-0",
  "approvalSignatures":["reviewer:test"],"derivedFromSnapshotId":"`+snap.ID+`","rulesetVersion":"rs-1",
  "finalizedAt":"2026-11-12T00:00:00Z",
  "actions":[{"id":"a1","targetNodeId":"`+node+`","findingId":"`+fid+`",
    "cryptoRuntime":"CRYPTO_RUNTIME_JCA","kind":"REMEDIATION_KIND_PROVIDER_INJECT",
    "providerChoice":"BC","providerClass":"org.bouncycastle.jce.provider.BouncyCastleProvider",
    "targetAlgorithm":"ML-KEM (FIPS 203)","automationLevel":"DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL",
    "evidenceSources":[{"findingId":"`+fid+`"}]}]}`))

	bin := filepath.Join(t.TempDir(), "pqcota-provision")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/randyinthedev-hash/pqcota-provisioning/cmd/pqcota-provision").CombinedOutput(); err != nil {
		t.Fatalf("build pqcota-provision: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "--level", "l2", "--allow-unverified-approvals", "--allow-incomplete", "--dsn", dsn, plan)
	cmd.Env = append(os.Environ(), "PQCOTA_ORG="+organization, "PQCOTA_APPROVAL_KEYS=")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("pqcota-provision: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "hosts:") {
		t.Errorf("no playbook was generated:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "before not captured 1") {
		t.Errorf("the summary does not count the uncaptured before:\n%s", stderr.String())
	}

	store, err := provisioning.NewPgRecordStoreIn(context.Background(), dsn, organization)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	recs, err := store.ByNode(node)
	if err != nil || len(recs) != 1 {
		t.Fatalf("records: %v %d", err, len(recs))
	}
	rec := recs[0]
	if len(rec.GetBefore().GetProviderChain()) != 0 || !strings.Contains(rec.GetNote(), "observed only through the JVM selected with --pid") {
		t.Errorf("record before %v note %q", rec.GetBefore(), rec.GetNote())
	}
}
