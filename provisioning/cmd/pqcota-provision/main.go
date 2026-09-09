// Command pqcota-provision — 확정 계획(FinalizedPlan JSON)에서 프로비저닝 산출물을 만든다:
//
//	(1) L1/L2 Ansible 플레이북 생성(프로비저닝 설계 §4.1 — core는 생성만, 실행은 사용자 Ansible)
//	    --rollback 시 역방향(롤백) 플레이북: forward가 배치한 파일 제거(§6A).
//	(2) 조치별 before 상태 캡처 → append-only 레코드로 영속(§6A 롤백 근거)
//
// L3(--level l3)는 계획의 activation 훅(사용자가 적은 비활성화·활성화·재시작 명령)을 의미 순서로
// 플레이북에 배치한다. 활성화 방법을 도구가 추측하지 않는다(§2.5).
//
// usage: pqcota-provision [--level l1|l2|l3] [--rollback] [--dsn <postgres>] <plan.json>
//
//	--dsn 지정 시: 히스토리에서 before-findings를 읽어 레코드를 캡처·영속(같은 저장소).
//	미지정 시: 플레이북만 stdout(레코드 없음).
//	env PQCOTA_APPROVAL_KEYS   : (선택) `<승인자>=<base64 공개키>` 콤마 구분. 있으면 승인 서명을
//	                             **그 승인자의 키로** 검증한다(§3.3③). 하나라도 어긋나면 거절한다.
//	env PQCOTA_REQUIRE_APPROVAL: "1"이면 검증할 키가 없을 때 **생성을 시작하지 않는다.**
//	                             조용히 통과하는 경로를 닫아야 하는 배포용(§2.6).
//
// 승인 서명은 pqcota-approve가 붙인다. 키쌍은 pqcota-keygen이 낸다.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	provisioningv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/provisioning/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/discovery/history"
	"github.com/randyinthedev-hash/pqcota/pkg/kernel/sign"
	"github.com/randyinthedev-hash/pqcota/pkg/org"
	"github.com/randyinthedev-hash/pqcota/pkg/provisioning"
	"google.golang.org/protobuf/encoding/protojson"

	discoveryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/discovery/v1"
)

func main() {
	levelFlag := flag.String("level", "l2", "automation level: l1 (stage only) | l2 (through install) | l3 (through activation and restart, using the plan's activation hooks)")
	rollbackFlag := flag.Bool("rollback", false, "generate the reverse (rollback) playbook — removes the files the forward run staged")
	dsn := flag.String("dsn", "", "Postgres DSN for history and records; when given, captures the before state and persists it")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pqcota-provision [--level l1|l2|l3] [--rollback] [--dsn <postgres>] <plan.json>")
		os.Exit(2)
	}

	raw, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "read plan:", err)
		os.Exit(1)
	}
	plan := &provisioningv1.FinalizedPlan{}
	if err := protojson.Unmarshal(raw, plan); err != nil {
		fmt.Fprintln(os.Stderr, "parse plan:", err)
		os.Exit(1)
	}

	// §3.7 최강 게이트 — 실행 근거인지 판정하는 규칙은 provisioning.Executable 하나뿐이다.
	// 여기서 조건을 다시 적으면 규칙이 두 곳에 생기고, 실제로 그렇게 갈렸다: 상태만 비교하던
	// 동안 승인 서명·조치가 빈 FINALIZED 계획이 그대로 통과했다. 사유를 함께 싣는다 — 무엇이
	// 모자란지 말하지 않고 거절하면 사용자가 계획을 고칠 수 없다.
	if err := provisioning.Executable(plan); err != nil {
		// 꼬리말은 사유에 맞춰 붙인다. 조치 내용이 모자란 계획은 **이미 확정된** 것이라,
		// "확정된 계획만 근거가 된다"고 덧붙이면 고칠 자리를 잘못 가리킨다.
		tail := "Fill the action in — a finalized plan is the only grounds, and it has to say what to do (§3.7)."
		if errors.Is(err, provisioning.ErrNotFinalized) {
			tail = "Only a finalized plan justifies provisioning (§3.7)."
		}
		fmt.Fprintf(os.Stderr, "refused: %v. %s\n", err, tail)
		os.Exit(1)
	}

	// 승인 서명 검증(§3.3③) — Executable은 서명의 **개수**만 센다. 값이 맞는지, 누구 것인지는
	// 여기서 본다. 키 묶음이 없으면 확인할 수 없으므로 **확인했다고 하지 않는다**(§2.6).
	if err := checkApprovals(plan); err != nil {
		fmt.Fprintln(os.Stderr, "refused:", err)
		os.Exit(1)
	}

	level := provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL
	switch *levelFlag {
	case "l1":
		level = provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L1_STAGE_ONLY
	case "l3":
		level = provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L3_FULL_AUTO
	}

	// (1) 플레이북 — stdout. --rollback이면 역방향(배치 파일 제거), 아니면 forward.
	if *rollbackFlag {
		fmt.Print(provisioning.GenerateRollbackPlaybook(plan, level))
		return
	}
	fmt.Print(provisioning.GenerateProvisioningPlaybook(plan, level))

	// 산출물이 그대로는 불완전한 조치(JCA provider_class 미확정 → java.security placeholder)를
	// 조용히 통과시키지 않는다 — 조각 안 ⚠는 열어봐야 보이므로 여기서 stderr로 크게 알린다(§2.5).
	for _, w := range provisioning.ProviderClassWarnings(plan) {
		fmt.Fprintln(os.Stderr, "⚠ [provision] "+w)
	}
	// provider 주입은 java.security의 한 자리를 대체한다 — 무엇이 밀려나는지 알린다.
	for _, w := range provisioning.ProviderSlotWarnings(plan) {
		fmt.Fprintln(os.Stderr, "⚠ [provision] "+w)
	}
	// 같은 런타임에 조각이 여러 개면 경로를 나눴다는 사실을 알린다 — 나눈 채 두면 참조되지 않는다.
	for _, w := range provisioning.ConfigConflictWarnings(plan) {
		fmt.Fprintln(os.Stderr, "⚠ [provision] "+w)
	}
	// L3인데 훅이 비면 무엇이 **일어나지 않는지** 알린다 — 활성화 방법을 추측하지 않기 때문(§2.5).
	for _, w := range provisioning.ActivationWarnings(plan, level) {
		fmt.Fprintln(os.Stderr, "⚠ [provision] "+w)
	}
	// 목표 알고리즘이 그룹으로 안 풀리면 조각의 Groups 줄이 주석으로 나간다 — 배치해도 아무것도
	// 켜지지 않는데, 그 사실이 조각 안에만 적혀 있어 열어보지 않으면 모른다.
	for _, w := range provisioning.TargetAlgorithmWarnings(plan) {
		fmt.Fprintln(os.Stderr, "⚠ [provision] "+w)
	}
	// 무엇에서 뽑은 계획인지 되짚을 수 있는가(§1.2). 실행은 되지만 이력에 근거가 안 남는다.
	for _, w := range provisioning.TraceabilityWarnings(plan) {
		fmt.Fprintln(os.Stderr, "⚠ [provision] "+w)
	}

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "[provision] no --dsn → skipping the before capture and record persistence (playbook only).")
		return
	}

	// (2) before 캡처 + 레코드 영속.
	ctx := context.Background()
	hist, err := history.NewPgStoreIn(ctx, *dsn, org.FromEnv())
	if err != nil {
		fmt.Fprintln(os.Stderr, "connecting to history:", err)
		os.Exit(1)
	}
	defer hist.Close()
	recs, err := provisioning.NewPgRecordStore(ctx, *dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "record store:", err)
		os.Exit(1)
	}
	defer recs.Close()

	// 노드별 최신 findings 캐시(before 기준·app_key 부착).
	byNode := map[string][]*discoveryv1.Finding{}
	findingByID := map[string]*discoveryv1.Finding{}
	load := func(node string) []*discoveryv1.Finding {
		if fs, ok := byNode[node]; ok {
			return fs
		}
		snap, err := hist.Latest(node)
		if err != nil {
			fmt.Fprintln(os.Stderr, "reading history:", err)
			os.Exit(1)
		}
		var fs []*discoveryv1.Finding
		if snap != nil {
			fs = snap.Findings
			for _, f := range fs {
				findingByID[f.GetId()] = f
			}
		}
		byNode[node] = fs
		return fs
	}

	n := 0
	for _, a := range plan.GetActions() {
		node := a.GetTargetNodeId()
		before := load(node)
		var appKeys []string // 근거 Finding의 자산이 어느 앱 것인지(§1.5) — 공유 .so면 다중
		if f := findingByID[a.GetFindingId()]; f != nil {
			appKeys = f.GetAppKeys()
		}
		rec := provisioning.NewProvisioningRecord(
			plan.GetId()+":"+a.GetId(), node, appKeys, plan.GetId(), a, before)
		if err := recs.Append(rec); err != nil {
			fmt.Fprintln(os.Stderr, "appending a record:", err)
			os.Exit(1)
		}
		n++
	}
	fmt.Fprintf(os.Stderr, "[provision] persisted %d records (before capture · STAGED · rollback basis).\n", n)
}

// checkApprovals — 승인 서명이 등록된 승인자의 것인지 확인한다.
//
// PQCOTA_APPROVAL_KEYS가 `<승인자>=<base64 공개키>` 묶음이다. **키만 나열하지 않는 이유**가
// PQCOTA_VERIFY_KEY의 교훈이다: 키 목록은 "누군가 서명했다"까지만 답해서 어느 서명이 누구
// 것인지 말하지 못한다. 승인은 책임의 소재라 그 답으로는 부족하다.
//
// 키가 없으면 막지 않되 **확인하지 않았다고 크게 말한다.** 확인 못 한 것을 통과와 같은 자리에
// 두지 않는다(§2.6). 그 경로를 닫아야 하는 배포에서는 PQCOTA_REQUIRE_APPROVAL=1로 막는다.
func checkApprovals(plan *provisioningv1.FinalizedPlan) error {
	keys, err := sign.ParseKeyMap(os.Getenv("PQCOTA_APPROVAL_KEYS"))
	if err != nil {
		return fmt.Errorf("PQCOTA_APPROVAL_KEYS: %w", err)
	}
	if len(keys) == 0 {
		if os.Getenv("PQCOTA_REQUIRE_APPROVAL") == "1" {
			return fmt.Errorf("PQCOTA_REQUIRE_APPROVAL=1 but PQCOTA_APPROVAL_KEYS is empty — there is no key to check the approvals with")
		}
		fmt.Fprintf(os.Stderr, "⚠ [provision] approval signatures: **not checked** — no PQCOTA_APPROVAL_KEYS to check them with. %d entries were counted, not verified.\n",
			len(plan.GetApprovalSignatures()))
		fmt.Fprintln(os.Stderr, "           where that is not good enough, close it with PQCOTA_REQUIRE_APPROVAL=1.")
		return nil
	}

	chk := sign.VerifyApprovals(keys, plan)
	for _, u := range chk.Unverifiable {
		// 이름표는 틀린 것이 아니라 **아무것도 증명하지 않는 것**이다. 거부와 다른 칸에 둔다.
		fmt.Fprintf(os.Stderr, "⚠ [provision] approval %q is not a signature — it is a label and proves nothing. Sign it with pqcota-approve.\n", u)
	}
	if len(chk.Rejected) > 0 {
		var names []string
		for _, r := range chk.Rejected {
			names = append(names, r.String())
		}
		return fmt.Errorf("approval signatures did not check out: %s. A plan carrying an approval that is not the approver's is worse than one carrying none (§3.3③)",
			strings.Join(names, ", "))
	}
	if len(chk.Approved) == 0 {
		return fmt.Errorf("no approval on this plan could be verified with the registered keys, so nothing shows who approved it (§3.3③)")
	}
	fmt.Fprintf(os.Stderr, "[provision] approvals verified: %s\n", strings.Join(chk.Approved, ", "))
	return nil
}
