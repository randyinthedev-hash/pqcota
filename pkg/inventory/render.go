// Package inventory renders the read-only inventory view (설계 §8-7, Phase 0 산출물).
// 스냅샷의 파생 Finding + 완전성 갭을 사람이 읽는 텍스트로. 판단은 하지 않는다(§2.1).
package inventory

import (
	"fmt"
	"sort"
	"strings"

	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/history"
	"github.com/pqcota/pqcota/pkg/kernel/posture"
)

// Render — 읽기전용 인벤토리 뷰. 갭은 "부재"가 아니라 "원리상 못 봄"으로 명시(§2.7).
func Render(snap *history.Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "노드 %s  (스냅샷 %s · ruleset %s)\n", snap.NodeID, snap.ID, snap.RulesetVersion)
	fmt.Fprintf(&b, "%-8s %-12s %-22s %-28s %s\n", "runtime", "evidence", "detection", "detail", "id")
	for _, f := range snap.Findings {
		detail := detailOf(f)
		id := shortID(f.GetId())
		fmt.Fprintf(&b, "%-8s %-12s %-22s %-28s %s\n",
			short(f.GetCryptoRuntime().String(), "CRYPTO_RUNTIME_"),
			short(f.GetEvidenceStrength().String(), "EVIDENCE_STRENGTH_"),
			short(f.GetDetectionMethod().String(), "DETECTION_METHOD_"),
			detail, id)
	}
	if snap.ExcludedByScope > 0 {
		// 제외는 "없음"이 아니다 — 정책으로 뺀 걸 감추면 인벤토리가 거짓말한다(§2.7).
		fmt.Fprintf(&b, "자산 스코프 제외: %d건 (관측됐으나 관리 대상 아님 — 부재가 아니다)\n", snap.ExcludedByScope)
	}
	if c := snap.Completeness; c != nil && len(c.GetLayersMissing()) > 0 {
		var g []string
		for _, l := range c.GetLayersMissing() {
			g = append(g, short(l.String(), "COLLECTION_LAYER_"))
		}
		fmt.Fprintf(&b, "갭(원리상 못 봄 ≠ 부재): %s  %s\n", strings.Join(g, ","), c.GetNote())
	}
	return b.String()
}

// detailOf — finding의 런타임별 상세 한 줄. Render와 diff가 같은 문자열을 봐야
// "무엇이 달라졌나"가 화면과 어긋나지 않는다.
func detailOf(f *discoveryv1.Finding) string {
	detail := "-"
	switch {
	case f.GetOpenssl() != nil:
		detail = fmt.Sprintf("%s/%s %s [%s]", f.GetOpenssl().GetLib(), nz(f.GetOpenssl().GetFork()),
			f.GetOpenssl().GetVersion(), short(f.GetOpenssl().GetBindingMode().String(), "OPENSSL_BINDING_MODE_"))
	case f.GetJca() != nil:
		detail = fmt.Sprintf("providers=%d %s", len(f.GetJca().GetProviderSet()), f.GetPqcReadiness())
	}
	if ak := f.GetAppKeys(); len(ak) > 0 {
		detail += "  @" + strings.Join(ak, ",") // 자산 귀속(§0.5) — 공유 .so면 다중 앱
	}
	return detail
}

func shortID(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func gapOf(snap *history.Snapshot) string {
	c := snap.Completeness
	if c == nil || len(c.GetLayersMissing()) == 0 {
		return "-"
	}
	var g []string
	for _, l := range c.GetLayersMissing() {
		g = append(g, short(l.String(), "COLLECTION_LAYER_"))
	}
	return strings.Join(g, ",")
}

// RenderHistory — 한 노드의 **변화 지점**을 오래된 것부터 나열한다. 스냅샷은 실질 내용이
// 바뀔 때만 쌓이므로, 각 줄은 "이 상태였던 구간"이고 obs 열이 그동안 몇 번 관측했는지 말한다.
// 관측 사실 서술이지 판정이 아니다 — 변화가 선언과 맞는지·의미 있는지는 판정이라 하지 않는다(아키텍처 §6).
func RenderHistory(nodeID string, snaps []*history.Snapshot, stats map[string]history.ObsStat,
	pruned []history.RetentionEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "노드 %s — 변화 지점 %d건 (오래된 것부터)\n", nodeID, len(snaps))
	// 절단이 있었으면 먼저 고지한다 — 안 그러면 이력의 구멍이 "관측을 안 함"으로 읽힌다(§2.7 정신).
	for _, e := range pruned {
		fmt.Fprintf(&b, "⌫ %s 이전 %d건은 보존 정책으로 절단됨 (%s, 관측 기록 %d건 · %s 실행)\n",
			e.PrunedUpTo.Format("2006-01-02 15:04:05"), e.Snapshots, e.Policy, e.Observations,
			e.ExecutedAt.Format("2006-01-02"))
	}
	b.WriteByte('\n')
	if len(snaps) == 0 {
		b.WriteString("(적재된 스냅샷 없음)\n")
		return b.String()
	}
	// 컬럼 헤더는 영문 — 한글은 터미널에서 2배 폭이라 고정폭 칸 정렬이 깨진다(Render와 같은 방식).
	// snapshot은 길이가 들쭉날쭉하고 **자르면 안 되는** 값(사용자가 -snapshot·-diff에 그대로 붙여넣는
	// 손잡이)이라 맨 뒤에 통째로 둔다.
	fmt.Fprintf(&b, "%-6s %-19s %-13s %8s %6s %5s %-24s %-18s %s\n",
		"seq", "changed", "ruleset", "findings", "edges", "obs", "observed", "gap", "snapshot")
	for _, s := range snaps {
		obs, window := "-", "-"
		if st, ok := stats[s.ID]; ok {
			obs = fmt.Sprintf("%d", st.Count)
			window = st.First.Format("01-02 15:04") + "~" + st.Last.Format("01-02 15:04")
		}
		fmt.Fprintf(&b, "%-6d %-19s %-13s %8d %6d %5s %-24s %-18s %s\n",
			s.Seq, s.CreatedAt.Format("2006-01-02 15:04:05"), s.RulesetVersion,
			len(s.Findings), len(s.Edges), obs, window, gapOf(s), s.ID)
	}
	b.WriteString("\n(스냅샷은 내용이 바뀔 때만 쌓인다 — obs·observed가 그 상태를 몇 번·언제까지 재확인했는지 말한다.\n")
	b.WriteString(" gap = 원리상 못 본 계층으로 \"부재\"가 아니다. snapshot 값을 -snapshot·-diff에 그대로 쓴다.)\n")
	return b.String()
}

// RenderDetail — 스냅샷 단건 상세: 자산 뷰 + 그 스냅샷의 관측 엣지.
// (Render는 누적 뷰가 쓰므로 건드리지 않고, 엣지는 상세에서만 펼친다.)
func RenderDetail(snap *history.Snapshot) string {
	var b strings.Builder
	b.WriteString(Render(snap))
	if len(snap.Edges) == 0 {
		return b.String()
	}
	fmt.Fprintf(&b, "\n관측 엣지 %d (이 스냅샷)\n", len(snap.Edges))
	for _, e := range snap.Edges {
		fmt.Fprintf(&b, "  %s → %-22s %-5s %-38s %s\n",
			e.GetSrcNodeId(), edgeDst(e), short(e.GetProtocol().String(), "NETWORK_PROTOCOL_"),
			edgeAlgo(e), postureMark(e))
	}
	return b.String()
}

// RenderDiff — 두 스냅샷 사이의 변화를 관측 사실로만 서술한다(추가·사라짐·변경).
// finding id는 (node|name|runtime|fork) 해시라 버전이 바뀌어도 유지된다 → 같은 자산의 변경으로 잡힌다.
func RenderDiff(a, b *history.Snapshot) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "변화 (노드 %s)\n  %s\n  → %s\n", b.NodeID, a.ID, b.ID)
	fmt.Fprintf(&sb, "  %s (seq %d, ruleset %s)  →  %s (seq %d, ruleset %s)\n\n",
		a.CreatedAt.Format("2006-01-02 15:04:05"), a.Seq, a.RulesetVersion,
		b.CreatedAt.Format("2006-01-02 15:04:05"), b.Seq, b.RulesetVersion)
	if a.RulesetVersion != b.RulesetVersion {
		sb.WriteString("⚠ ruleset이 다르다 — 파생값(evidence·pqc_readiness) 차이는 실제 변화가 아니라 재계산 결과일 수 있다(§0.2).\n\n")
	}
	// 방향 규약: 첫 인자=과거, 둘째=최신. '추가'=둘째에만·'사라짐'=첫째에만이라, 인자를 시간
	// 역순으로 주면 방향이 뒤집혀 읽힌다 — 하드 에러는 아니다(되돌림 미리보기로 역순 비교가
	// 유효하므로). 대신 뒤집혔음을 고지한다(§2.7 — 오독을 조용히 두지 않는다).
	if a.CreatedAt.After(b.CreatedAt) {
		sb.WriteString("⚠ 첫 스냅샷이 더 최신이다(시간 역순) — '추가'는 실은 사라진 것, '사라짐'은 생긴 것으로 뒤집혀 읽힌다. 시간순은 <과거id>,<최신id>.\n\n")
	}

	am, bm := indexFindings(a.Findings), indexFindings(b.Findings)
	var added, removed, changed []string
	for _, id := range sortedKeys(bm) {
		f := bm[id]
		old, ok := am[id]
		switch {
		case !ok:
			added = append(added, fmt.Sprintf("  + %-8s %s  %s",
				short(f.GetCryptoRuntime().String(), "CRYPTO_RUNTIME_"), detailOf(f), shortID(id)))
		case detailOf(old) != detailOf(f) || old.GetEvidenceStrength() != f.GetEvidenceStrength():
			changed = append(changed, fmt.Sprintf("  ~ %s  %s\n      → %s", shortID(id), diffLine(old), diffLine(f)))
		}
	}
	for _, id := range sortedKeys(am) {
		if _, ok := bm[id]; !ok {
			f := am[id]
			removed = append(removed, fmt.Sprintf("  - %-8s %s  %s",
				short(f.GetCryptoRuntime().String(), "CRYPTO_RUNTIME_"), detailOf(f), shortID(id)))
		}
	}

	section(&sb, "추가", added)
	section(&sb, "사라짐", removed)
	section(&sb, "변경", changed)
	if len(added)+len(removed)+len(changed) == 0 {
		fmt.Fprintf(&sb, "자산 변화 없음 (양쪽 %d건 동일)\n", len(bm))
	}
	if la, lb := len(a.Edges), len(b.Edges); la != lb {
		fmt.Fprintf(&sb, "\n관측 엣지: %d → %d\n", la, lb)
	}
	if ga, gb := gapOf(a), gapOf(b); ga != gb {
		fmt.Fprintf(&sb, "갭(원리상 못 봄): %s → %s\n", ga, gb)
	}
	return sb.String()
}

func section(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "%s %d\n", title, len(lines))
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	b.WriteByte('\n')
}

func diffLine(f *discoveryv1.Finding) string {
	return fmt.Sprintf("%s [%s]", detailOf(f), short(f.GetEvidenceStrength().String(), "EVIDENCE_STRENGTH_"))
}

func indexFindings(fs []*discoveryv1.Finding) map[string]*discoveryv1.Finding {
	m := make(map[string]*discoveryv1.Finding, len(fs))
	for _, f := range fs {
		m[f.GetId()] = f
	}
	return m
}

func sortedKeys(m map[string]*discoveryv1.Finding) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out) // 결정론적 출력
	return out
}

func edgeDst(e *discoveryv1.ObservedEdge) string {
	d := e.GetDstNodeId()
	if d == "" {
		return e.GetDstAddr() // 스코프 미해소(off-scope) — 원시 주소로 표기(§0.4)
	}
	if p := e.GetPort(); p != 0 {
		return fmt.Sprintf("%s:%d", d, p)
	}
	return d
}

func edgeAlgo(e *discoveryv1.ObservedEdge) string {
	g, c := e.GetNegotiatedGroup(), e.GetCipher()
	switch {
	case g != "" && c != "":
		return g + " / " + c
	case g != "":
		return g
	case c != "":
		return c
	}
	return "-"
}

func postureMark(e *discoveryv1.ObservedEdge) string {
	switch posture.Classify(e.GetNegotiatedGroup(), e.GetCipher()) {
	case discoveryv1.QuantumPosture_QUANTUM_POSTURE_PQC_HYBRID:
		return "🟢 PQC/하이브리드"
	case discoveryv1.QuantumPosture_QUANTUM_POSTURE_CLASSICAL:
		return "🔴 고전"
	default:
		return "⚪ 불명"
	}
}

func short(s, prefix string) string { return strings.ToLower(strings.TrimPrefix(s, prefix)) }

func nz(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
