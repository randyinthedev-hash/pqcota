// Command pqcota-provision — 확정 계획(FinalizedPlan JSON)에서 프로비저닝 산출물을 만든다:
//
//	(1) L1/L2 Ansible 플레이북 생성(§4.3 — core는 생성만, 실행은 사용자 Ansible)
//	    --rollback 시 역방향(롤백) 플레이북: forward가 배치한 파일 제거(§6A).
//	(2) 조치별 before 상태 캡처 → append-only 레코드로 영속(§6A 롤백 근거)
//
// L3(--level l3)는 계획의 activation 훅(사용자가 적은 비활성화·활성화·재시작 명령)을 의미 순서로
// 플레이북에 배치한다. 활성화 방법을 도구가 추측하지 않는다(§2.6).
//
// usage: pqcota-provision [--level l1|l2|l3] [--rollback] [--dsn <postgres>] <plan.json>
//
//	--dsn 지정 시: 히스토리에서 before-findings를 읽어 레코드를 캡처·영속(같은 저장소).
//	미지정 시: 플레이북만 stdout(레코드 없음).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	provisioningv1 "github.com/pqcota/pqcota/gen/pqcota/provisioning/v1"
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/provisioning"
	"google.golang.org/protobuf/encoding/protojson"

	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
)

func main() {
	levelFlag := flag.String("level", "l2", "배포 자동화 수준: l1(스테이지만) | l2(설치까지) | l3(활성화·재시작까지 — 계획의 activation 훅 사용)")
	rollbackFlag := flag.Bool("rollback", false, "역방향(롤백) 플레이북 생성 — forward가 배치한 파일 제거")
	dsn := flag.String("dsn", "", "히스토리·레코드 Postgres DSN(지정 시 before 캡처·영속)")
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

	// §3.7 최강 게이트 — FINALIZED 아니면 실행 근거 없음.
	if plan.GetStatus() != provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED {
		fmt.Fprintf(os.Stderr, "거부: 계획이 FINALIZED 아님(%s). 프로비저닝 근거는 확정 계획뿐(§3.7).\n", plan.GetStatus())
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
	// 조용히 통과시키지 않는다 — 조각 안 ⚠는 열어봐야 보이므로 여기서 stderr로 크게 알린다(§2.6).
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
	// L3인데 훅이 비면 무엇이 **일어나지 않는지** 알린다 — 활성화 방법을 추측하지 않기 때문(§2.6).
	for _, w := range provisioning.ActivationWarnings(plan, level) {
		fmt.Fprintln(os.Stderr, "⚠ [provision] "+w)
	}

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "[provision] --dsn 미지정 → before 캡처·레코드 영속 생략(플레이북만).")
		return
	}

	// (2) before 캡처 + 레코드 영속.
	ctx := context.Background()
	hist, err := history.NewPgStore(ctx, *dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "히스토리 연결:", err)
		os.Exit(1)
	}
	defer hist.Close()
	recs, err := provisioning.NewPgRecordStore(ctx, *dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "레코드 저장소:", err)
		os.Exit(1)
	}
	defer recs.Close()

	// 노드별 최신 findings 캐시(before 기준·app_key 귀속).
	byNode := map[string][]*discoveryv1.Finding{}
	findingByID := map[string]*discoveryv1.Finding{}
	load := func(node string) []*discoveryv1.Finding {
		if fs, ok := byNode[node]; ok {
			return fs
		}
		snap, err := hist.Latest(node)
		if err != nil {
			fmt.Fprintln(os.Stderr, "히스토리 조회:", err)
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
		var appKeys []string // 근거 Finding의 자산 귀속(§4A.2) — 공유 .so면 다중
		if f := findingByID[a.GetFindingId()]; f != nil {
			appKeys = f.GetAppKeys()
		}
		rec := provisioning.NewProvisioningRecord(
			plan.GetId()+":"+a.GetId(), node, appKeys, plan.GetId(), a, before)
		if err := recs.Append(rec); err != nil {
			fmt.Fprintln(os.Stderr, "레코드 append:", err)
			os.Exit(1)
		}
		n++
	}
	fmt.Fprintf(os.Stderr, "[provision] 레코드 %d개 영속(before 캡처 · STAGED · 롤백 근거).\n", n)
}
