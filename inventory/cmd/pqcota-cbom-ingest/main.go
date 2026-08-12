// Command pqcota-cbom-ingest — 외부(CBOMkit 등) CycloneDX CBOM을 수신·검증·적재한다(SV-2·SD-7).
// 위임 수신(② 위임)의 종단 진입점: 소스·아티팩트를 pqcota가 스캔하지 않고, 사용자 CI가 낸
// 표준 CycloneDX를 받아 관측 레인(detection_method=source/artifact)으로 히스토리에 적재한다.
//
// 검증은 이 커맨드 안에서 강제된다(ImportCBOM 내부): (1) 서명(옵션) → (2) 구조 → (3) 앵커.
// 부적합 CBOM은 저장되지 않으므로 별도 프리플라이트 커맨드가 필요 없다.
//
// usage: pqcota-cbom-ingest <cbom.json | -> <target-node-id>
//
//	<cbom.json | ->   : CycloneDX CBOM 파일(또는 stdin '-')
//	<target-node-id>  : 이 CBOM을 귀속할 스코프 노드 ID(§1.4 앵커). 없으면 스코프 판정 요청(SD-5).
//	env PQCOTA_DSN     : (선택) 있으면 Postgres 영속화, 없으면 인메모리(요약만).
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/inventory/ingest"
	"github.com/pqcota/pqcota/pkg/org"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: pqcota-cbom-ingest <cbom.json | -> <target-node-id>")
		os.Exit(2)
	}
	src, nodeID := os.Args[1], os.Args[2]

	var raw []byte
	var err error
	if src == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(src)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	store, closeFn, persistent := openStore()
	defer closeFn()

	// CBOM 서명 검증(verifySig)은 아직 미배선 — CBOM은 신뢰된 CI/전송 경로로 온다는 전제(SV-2).
	// (sign 패키지는 CollectionResult 전용. raw-CBOM 서명 검증은 후속.)
	prefix := "cbom-" + time.Now().UTC().Format("20060102T150405Z")
	disp, err := ingest.IngestCBOM(raw, nodeID, nil, prefix, "ruleset-demo", store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ingest:", err)
		os.Exit(1)
	}

	backing := "인메모리(요약만 — 프로세스 종료 시 소멸)"
	if persistent {
		backing = "Postgres(append-only 영속)"
	}
	switch disp {
	case ingest.Accepted:
		n := 0
		if snap, _ := store.Latest(nodeID); snap != nil {
			n = len(snap.Findings)
		}
		fmt.Println("╔══════════════════════════════════════════════════╗")
		fmt.Println("║  pqcota CBOM 수신 (위임 → 관측 레인 → 히스토리)     ║")
		fmt.Println("╚══════════════════════════════════════════════════╝")
		fmt.Printf("✓ 수용: node=%s · detection_method=source/artifact · 자산 %d개 · 저장소 %s\n", nodeID, n, backing)
	case ingest.NeedsScopeBinding:
		fmt.Fprintf(os.Stderr, "✗ 앵커 없음: <target-node-id>가 스코프 마스터에 필요(§1.4·SD-5)\n")
		os.Exit(1)
	default: // Rejected
		fmt.Fprintf(os.Stderr, "✗ 거부: CBOM 검증 실패(서명·구조·스펙 부적합, TV-CBOM-2)\n")
		os.Exit(1)
	}
}

func openStore() (history.Store, func(), bool) {
	dsn := os.Getenv("PQCOTA_DSN")
	if dsn == "" {
		mem, err := history.NewMemStoreIn(org.FromEnv())
		if err != nil {
			fmt.Fprintln(os.Stderr, "조직:", err)
			os.Exit(2)
		}
		return mem, func() {}, false
	}
	pg, err := history.NewPgStoreIn(context.Background(), dsn, org.FromEnv())
	if err == nil {
		return pg, pg.Close, true
	}
	// **폴백하지 않는 경우** — 조직을 요구하는 배포에서 인메모리로 내려앉으면, 적재된 줄 알았던
	// 것이 프로세스와 함께 사라진다. 영속을 요구한 것은 DSN을 준 쪽이다.
	if org.Required() {
		fmt.Fprintln(os.Stderr, "Postgres 연결 실패:", err)
		fmt.Fprintln(os.Stderr, "  "+org.RequireEnv+"=1이므로 인메모리로 대체하지 않는다 — 적재를 멈춘다.")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Postgres 연결 실패, 인메모리로 대체:", err)
	mem, merr := history.NewMemStoreIn(org.FromEnv())
	if merr != nil {
		fmt.Fprintln(os.Stderr, "조직:", merr)
		os.Exit(2)
	}
	return mem, func() {}, false
}
