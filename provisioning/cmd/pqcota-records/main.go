// Command pqcota-records — 영속된 프로비저닝 레코드(롤백 근거)를 조회한다(§6A).
// pqcota-provision이 append-only로 남긴 before/after·영향 앱·상태를 읽기전용으로 나열한다.
// 이 뷰는 "무엇이 언제 어떤 before로 스테이징됐나"만 보인다.
//
// usage: pqcota-records [node]        (env PQCOTA_DSN 필수 — pqcota-provision과 같은 저장소)
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	provisioningv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/provisioning/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/org"
	"github.com/randyinthedev-hash/pqcota/pkg/provisioning"
)

func main() {
	dsn := os.Getenv("PQCOTA_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "PQCOTA_DSN 필요 — pqcota-provision이 레코드를 적재한 Postgres.")
		os.Exit(2)
	}
	store, err := provisioning.NewPgRecordStoreIn(context.Background(), dsn, org.FromEnv())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Postgres 연결:", err)
		os.Exit(1)
	}
	defer store.Close()

	var recs []*provisioningv1.ProvisioningRecord
	if len(os.Args) > 1 && os.Args[1] != "" {
		recs, err = store.ByNode(os.Args[1])
	} else {
		recs, err = store.All()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "조회:", err)
		os.Exit(1)
	}

	fmt.Printf("═══ 프로비저닝 레코드 (%d) ═══\n", len(recs))
	for _, r := range recs {
		fmt.Printf("• %s  [%s]  node=%s  plan=%s\n", r.GetId(),
			short(r.GetStatus().String(), "PROVISIONING_STATUS_"), r.GetNodeId(), r.GetPlanId())
		if ak := r.GetAppKeys(); len(ak) > 0 {
			fmt.Printf("    영향 앱: %s\n", strings.Join(ak, ", "))
		}
		if b := r.GetBefore(); b != nil && len(b.GetModules()) > 0 {
			fmt.Printf("    before : %s\n", strings.Join(b.GetModules(), ", "))
		}
		if a := r.GetAfter(); a != nil && len(a.GetModules()) > 0 {
			fmt.Printf("    after  : %s\n", strings.Join(a.GetModules(), ", "))
		}
		if r.GetNote() != "" {
			fmt.Printf("    note   : %s\n", r.GetNote())
		}
	}
}

func short(s, prefix string) string { return strings.TrimPrefix(s, prefix) }
