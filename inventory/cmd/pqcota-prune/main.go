// Command pqcota-prune — 보존 정책 집행(오래된 변화 지점 절단).
//
// **조회 커맨드(pqcota-inventory)와 일부러 분리했다** — 읽기 도구가 파괴적 동작을 겸하면
// 실수 한 번이 이력을 지운다.
//
// 기본은 dry-run이다. 무엇을 얼마나 지울지 먼저 보이고, 실제 삭제는 -apply로만 한다.
//
//	pqcota-prune -keep-last 20                  # 계획만(아무것도 안 지움)
//	pqcota-prune -older-than 90d -apply         # 실제 절단
//	pqcota-prune -older-than 90d -keep-last 20  # 둘 다: 보수적 판정(둘 다 버려도 될 때만)
//
// 불변식: 노드별 **최신 스냅샷은 어떤 정책으로도 지우지 않는다**(인벤토리 뷰·프로비저닝
// before 캡처의 근거). 절단하면 그 사실이 기록으로 남아 이력의 구멍을 설명한다.
//
// env PQCOTA_DSN 필수.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/org"
)

func main() {
	olderThan := flag.String("older-than", "", "이보다 오래된 변화 지점을 절단 (예: 90d, 720h)")
	keepLast := flag.Int("keep-last", 0, "노드별 최근 N개 변화 지점은 보존")
	apply := flag.Bool("apply", false, "실제로 삭제한다 (기본은 계획만)")
	flag.Parse()

	dur, err := parseDuration(*olderThan)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	pol := history.Policy{OlderThan: dur, KeepLast: *keepLast}

	dsn := os.Getenv("PQCOTA_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "PQCOTA_DSN 필요 — 절단할 영속 저장소를 가리켜야 함.")
		os.Exit(2)
	}
	store, err := history.NewPgStoreIn(context.Background(), dsn, org.FromEnv())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Postgres 연결:", err)
		os.Exit(1)
	}
	defer store.Close()

	rep, err := store.Prune(pol, *apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(render(rep))
}

func render(rep *history.PruneReport) string {
	var b strings.Builder
	mode := "계획 (dry-run — 아무것도 지우지 않았다)"
	if rep.Applied {
		mode = "실행됨"
	}
	fmt.Fprintf(&b, "보존 정책 %s — %s\n\n", rep.Policy.String(), mode)
	if len(rep.Nodes) == 0 {
		b.WriteString("절단 대상 없음. (노드별 최신 스냅샷은 어떤 정책으로도 보존한다)\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%-24s %10s %10s  %s\n", "node", "snapshots", "observed", "~까지")
	for _, n := range rep.Nodes {
		fmt.Fprintf(&b, "%-24s %10d %10d  %s\n",
			n.NodeID, n.Snapshots, n.Observations, n.UpTo.Format("2006-01-02 15:04:05"))
	}
	s, o := rep.Total()
	fmt.Fprintf(&b, "\n합계: 변화 지점 %d건 · 관측 기록 %d건\n", s, o)
	if !rep.Applied {
		b.WriteString("실제로 지우려면 -apply 를 붙인다. 절단 사실은 기록으로 남아 이력에 고지된다.\n")
	}
	return b.String()
}

// parseDuration — time.ParseDuration에 일(day) 단위를 더한다("90d"). 보존 정책은 일 단위로
// 말하는 게 자연스러운데 표준 파서는 시간까지만 안다.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("-older-than 값이 잘못됨: %q (예: 90d, 720h)", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("-older-than 값이 잘못됨: %q (예: 90d, 720h)", s)
	}
	return d, nil
}
