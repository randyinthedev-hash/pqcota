// Command pqcota-hosts — 사용자 관리 hosts 파일(CSV)을 읽어 discovery 접근을 준비한다(§4A.3).
//
//	(a) 런타임 전용 Ansible 인벤토리 생성(--ansible-out; 비밀 포함, 미영속) → 이걸로 ansible-playbook 실행
//	(b) 안전 엔드포인트(node_id·name·ip·port, 비밀 제외)를 stdout에 요약 → 인벤토리 재사용·수정용
//
// 접근 비밀(계정·키)은 (a)에만 실리고 pqcota 인벤토리엔 적재하지 않는다.
// usage: pqcota-hosts [--ansible-out <path>] [--dsn <postgres>] <hosts.csv>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/pqcota/pqcota/pkg/inventory"
)

func main() {
	out := flag.String("ansible-out", "", "런타임 Ansible 인벤토리(ini) 출력 경로(미지정 시 생성 안 함)")
	dsn := flag.String("dsn", "", "인벤토리 Postgres DSN(지정 시 안전 엔드포인트를 upsert — 비밀 제외)")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pqcota-hosts [--ansible-out <path>] [--dsn <postgres>] <hosts.csv>")
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer f.Close()

	hosts, err := inventory.ParseHosts(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}

	if *out != "" {
		inv := inventory.RenderAnsibleInventory(hosts) // 비밀 포함 — 런타임 전용
		if err := os.WriteFile(*out, []byte(inv), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "write ansible:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[hosts] Ansible 인벤토리 생성(런타임 전용·0600): %s\n", *out)
	}

	// 안전 엔드포인트(비밀 없음) — 인벤토리 재사용·수정 대상.
	eps := inventory.Endpoints(hosts)
	fmt.Printf("엔드포인트 %d개 (인벤토리 적재 대상 — 비밀 제외):\n", len(hosts))
	for _, ep := range eps {
		fmt.Printf("  %-14s %-16s %s:%d\n", ep.GetNodeId(), ep.GetName(), ep.GetIp(), ep.GetPort())
	}

	// --dsn 지정 시 인벤토리에 upsert(재사용·사용자 수정 가능). 비밀은 여전히 미영속.
	if *dsn != "" {
		meta, err := inventory.NewPgMetaStore(context.Background(), *dsn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "메타 저장소:", err)
			os.Exit(1)
		}
		defer meta.Close()
		for _, ep := range eps {
			if err := meta.UpsertEndpoint(ep); err != nil {
				fmt.Fprintln(os.Stderr, "upsert:", err)
				os.Exit(1)
			}
		}
		fmt.Fprintf(os.Stderr, "[hosts] 엔드포인트 %d개 인벤토리 upsert 완료(비밀 제외)\n", len(eps))
	}
}
