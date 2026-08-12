// Command pqcota-inventory — 중앙에서 실행. pqcota-ingest가 적재한 append-only 히스토리를 읽어
// 누적 인벤토리(발견 자산 + 관측 엣지 posture)를 조회한다. 읽기전용·무판단(§2.1).
// 파일 취합(discover-view, 휘발성)과 달리 영속 저장소에서 읽으므로 Postgres가 필요하다.
//
// 이력 열람·스냅샷 간 변화 diff는 관측 사실 서술이라 이 리포 범위 안이다(아키텍처 §6 기준).
// 선언(CMDB) 대조·리뷰확정 판정은 하지 않는다.
//
//	pqcota-inventory                       # 전 노드 최신 누적 뷰
//	pqcota-inventory -history node-db      # 그 노드의 스냅샷 이력(오래된 것부터)
//	pqcota-inventory -snapshot <id>        # 스냅샷 단건 상세(자산 + 관측 엣지)
//	pqcota-inventory -diff <과거id>,<최신id> # 두 스냅샷 사이의 변화(첫=과거·둘째=최신; 역순이면 경고)
//
// env PQCOTA_DSN 필수 — pqcota-ingest와 같은 저장소.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/inventory"
	"github.com/pqcota/pqcota/pkg/org"
)

func main() {
	histNode := flag.String("history", "", "노드의 스냅샷 이력을 나열한다 (append-only 측정 로그)")
	snapID := flag.String("snapshot", "", "스냅샷 단건 상세 — 자산 + 관측 엣지")
	diffPair := flag.String("diff", "", `두 스냅샷 사이의 변화 — "과거id,최신id" (첫=과거·둘째=최신; 관측 사실만, 판정 아님)`)
	flag.Parse()

	dsn := os.Getenv("PQCOTA_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "PQCOTA_DSN 필요 — 적재(pqcota-ingest)와 같은 Postgres를 가리켜야 함.")
		fmt.Fprintln(os.Stderr, "(인메모리는 프로세스 간 공유가 안 되므로 조회 뷰는 영속 저장소 전제)")
		os.Exit(2)
	}
	store, err := history.NewPgStoreIn(context.Background(), dsn, org.FromEnv())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Postgres 연결:", err)
		os.Exit(1)
	}
	defer store.Close()

	out, err := run(store, dsn, *histNode, *snapID, *diffPair)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(out)
}

func run(store history.Store, dsn, histNode, snapID, diffPair string) (string, error) {
	switch {
	case histNode != "":
		snaps, err := store.Snapshots(histNode)
		if err != nil {
			return "", fmt.Errorf("이력 조회: %w", err)
		}
		stats, err := store.ObservationStats(histNode)
		if err != nil {
			return "", fmt.Errorf("관측 요약 조회: %w", err)
		}
		// 절단 기록은 있으면 곁들인다 — 조회 도구는 Pruner를 요구하지 않는다(파괴적 동작과 분리).
		var pruned []history.RetentionEvent
		if pr, ok := store.(history.Pruner); ok {
			if pruned, err = pr.RetentionEvents(histNode); err != nil {
				return "", fmt.Errorf("절단 기록 조회: %w", err)
			}
		}
		return inventory.RenderHistory(histNode, snaps, stats, pruned), nil

	case snapID != "":
		snap, err := mustSnapshot(store, snapID)
		if err != nil {
			return "", err
		}
		// 선언된 귀속을 **읽을 때만** 얹는다 — 저장된 관측 엣지는 그대로다.
		return inventory.RenderDetailWith(snap, declaredOverlay(store)), nil

	case diffPair != "":
		ids := strings.Split(diffPair, ",")
		if len(ids) != 2 || strings.TrimSpace(ids[0]) == "" || strings.TrimSpace(ids[1]) == "" {
			return "", fmt.Errorf(`-diff 는 "id1,id2" 형식이어야 한다 (받은 값: %q)`, diffPair)
		}
		a, err := mustSnapshot(store, strings.TrimSpace(ids[0]))
		if err != nil {
			return "", err
		}
		b, err := mustSnapshot(store, strings.TrimSpace(ids[1]))
		if err != nil {
			return "", err
		}
		if a.NodeID != b.NodeID {
			return "", fmt.Errorf("서로 다른 노드의 스냅샷은 비교하지 않는다 (%s vs %s)", a.NodeID, b.NodeID)
		}
		return inventory.RenderDiff(a, b), nil
	}

	// 기본 — 전 노드 최신 누적 뷰. 머신 메타데이터(엔드포인트·프로필)를 헤더에 곁들인다(§2.0).
	meta, err := inventory.NewPgMetaStoreIn(context.Background(), dsn, org.FromEnv())
	if err != nil {
		return "", fmt.Errorf("메타 저장소: %w", err)
	}
	defer meta.Close()
	out, err := inventory.RenderStore(store, meta)
	if err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	return out, nil
}

// declaredOverlay — 귀속 저장소에서 선언 색인을 만든다. 담지 못하는 저장소면 빈 색인이다.
func declaredOverlay(store history.Store) *inventory.AttributionOverlay {
	as, _ := store.(history.AttributionStore)
	return inventory.BuildAttributionOverlay(as)
}

func mustSnapshot(store history.Store, id string) (*history.Snapshot, error) {
	s, err := store.ByID(id)
	if err != nil {
		return nil, fmt.Errorf("스냅샷 조회(%s): %w", id, err)
	}
	if s == nil {
		return nil, fmt.Errorf("스냅샷 없음: %s", id)
	}
	return s, nil
}
