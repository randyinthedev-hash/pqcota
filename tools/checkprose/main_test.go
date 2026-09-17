package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func mustRules(t *testing.T, lines ...string) []rule {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "rules.tsv")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rs, err := loadRules(p)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func hitsOf(rel string, orig []byte, masked []byte, rs []rule) []hit {
	return match(rel, orig, masked, rs)
}

// useShippedOverlap — 실제로 함께 나가는 overlap.txt 를 쓴다. 테스트에만 목록을 적어 두면
// 파일이 비어도 케이스가 통과한다.
func useShippedOverlap(t *testing.T) {
	t.Helper()
	w, err := loadWords(filepath.Join("..", "..", "tools", "checkprose", overlapName))
	if err != nil {
		t.Fatal(err)
	}
	if len(w) == 0 {
		t.Fatal("overlap.txt 가 비어 있다")
	}
	old := overlap
	overlap = w
	t.Cleanup(func() { overlap = old })
}

// **코드 블록 안은 보지 않는다.**
//
// 지침이 「인용, 코드, 코드 주석에는 적용하지 않는다」고 못 박아 두었다. 코드 블록의
// 엠대시나 영어 낱말까지 세면 관문이 고칠 수 없는 것을 요구하게 되고, 그러면 아무도
// 켜지 않는다.
func TestFencedCodeIsNotCounted(t *testing.T) {
	rs := mustRules(t, "엠대시\t—\t콜론으로")
	src := []byte("본문에 하나 —\n```\n코드 안에 —\n```\n다시 본문\n")
	got := hitsOf("a.md", src, maskMarkdown(src), rs)
	if len(got) != 1 {
		t.Fatalf("코드 블록 안까지 셌다: %d건 %v", len(got), got)
	}
	if got[0].line != 1 {
		t.Errorf("줄 번호가 어긋났다: %d", got[0].line)
	}
}

// **인라인 코드도 덮는다.** 경로와 플래그가 백틱 안에 들어 있는 문서라, 덮지
// 않으면 `-machine` 같은 이름이 문체 위반으로 잡힌다.
func TestInlineCodeIsNotCounted(t *testing.T) {
	rs := mustRules(t, "머신\t머신\t기계")
	src := []byte("`머신` 은 코드다. 그런데 머신이라고 적었다.\n")
	got := hitsOf("a.md", src, maskMarkdown(src), rs)
	if len(got) != 1 {
		t.Fatalf("인라인 코드까지 셌다: %d건 %v", len(got), got)
	}
}

// **「헷갈리다」를 「갈리다」로 잡지 않는다.**
//
// RE2 에 뒤보기가 없어 「갈리는」 하나로 재면 「헷갈리는」이 함께 걸린다. 실제 문서에
// 그 자리가 여섯 곳 있었다. 잘못 잡는 관문은 목록에 예외를 쌓게 만들고, 예외가 쌓이면
// 진짜 위반도 함께 묻힌다.
func TestHetgalliDoesNotTripGalli(t *testing.T) {
	useShippedOverlap(t)
	rs := mustRules(t, "갈리다\t갈립니|갈리면|갈리는|갈려\t서로 다르다")
	src := []byte("헷갈리는 자리가 남습니다.\n말이 갈리면 곤란합니다.\n")
	got := hitsOf("a.md", src, maskMarkdown(src), rs)
	if len(got) != 1 {
		t.Fatalf("헷갈리는을 함께 잡았다: %d건 %v", len(got), got)
	}
	if got[0].line != 2 {
		t.Errorf("줄 번호가 어긋났다: %d", got[0].line)
	}
}

// **Go 는 문자열 리터럴만 본다.**
//
// 주석은 한국어로 적는 것이 이 리포의 규칙이다(CONTRIBUTING 「주석은 "왜"를 국문으로」).
// 주석까지 막으면 판단 근거를 적어 두는 방식이 통째로 막힌다.
func TestGoCommentsAreNotCounted(t *testing.T) {
	rs := mustRules(t, "엠대시\t—\t콜론으로")
	dir := t.TempDir()
	p := filepath.Join(dir, "text.go")
	src := "package ui\n\n// 주석에 엠대시 —\nvar s = \"화면 문구에 엠대시 —\"\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, masked, err := maskGo(p)
	if err != nil {
		t.Fatal(err)
	}
	got := hitsOf("text.go", orig, masked, rs)
	if len(got) != 1 {
		t.Fatalf("주석까지 셌다: %d건 %v", len(got), got)
	}
	if got[0].line != 4 {
		t.Errorf("줄 번호가 어긋났다: %d", got[0].line)
	}
}

// **덮어도 줄 번호와 원문이 살아 있다.**
//
// 덮기가 바이트 수를 바꾸면 걸린 자리를 알려 줄 수 없다. 그리고 보여 주는 글은 덮은 것이
// 아니라 원문이라야 한다: 덮인 줄을 찍으면 어느 문장인지 알아볼 수 없다.
func TestMaskingKeepsOffsetsAndShowsTheOriginal(t *testing.T) {
	for _, src := range []string{
		"가나다 `코드` 라라라 —\n",
		"```\n덮이는 줄\n```\n본문 —\n",
	} {
		if got := len(maskMarkdown([]byte(src))); got != len(src) {
			t.Errorf("바이트 수가 달라졌다: %d → %d (%q)", len(src), got, src)
		}
	}
	rs := mustRules(t, "엠대시\t—\t콜론으로")
	src := []byte("앞줄\n`코드` 를 쓴 줄에 엠대시 —\n")
	got := hitsOf("a.md", src, maskMarkdown(src), rs)
	if len(got) != 1 {
		t.Fatalf("%d건 %v", len(got), got)
	}
	if !strings.Contains(got[0].text, "`코드`") {
		t.Errorf("덮인 줄을 보여 줬다: %q", got[0].text)
	}
}

// **기준선보다 늘면 막는다.** 이 관문이 있는 이유가 이 한 줄이다.
func TestGrowingPastTheBaselineFails(t *testing.T) {
	base := map[key]int{{"엠대시", "a.md"}: 3}
	now := map[key]int{{"엠대시", "a.md"}: 5}
	grown, shrunk := compare(base, now)
	if len(grown) != 1 {
		t.Fatalf("늘어난 것을 못 잡았다: %v", grown)
	}
	if len(shrunk) != 0 {
		t.Errorf("줄지 않았는데 줄었다고 했다: %v", shrunk)
	}
	if !strings.Contains(grown[0], "a.md") || !strings.Contains(grown[0], "+2") {
		t.Errorf("무엇이 얼마나 늘었는지 말하지 않는다: %s", grown[0])
	}
}

// **줄어도 막는다.**
//
// 고쳐 놓고 기준선을 안 내리면 그 자리가 도로 채워져도 알 수 없다. 「절차의 한 걸음」을
// 하루 걷어냈다가 다음 날 릴리스 노트에 되살린 적이 있어, 내려가는 쪽도 잠근다.
func TestShrinkingAlsoFailsUntilTheBaselineMovesDown(t *testing.T) {
	base := map[key]int{{"조용히", "a.md"}: 4}
	now := map[key]int{}
	grown, shrunk := compare(base, now)
	if len(grown) != 0 {
		t.Errorf("늘지 않았는데 늘었다고 했다: %v", grown)
	}
	if len(shrunk) != 1 {
		t.Fatalf("줄어든 것을 못 잡았다: %v", shrunk)
	}
	if !strings.Contains(shrunk[0], "-4") {
		t.Errorf("얼마나 줄었는지 말하지 않는다: %s", shrunk[0])
	}
}

// **기준선을 찍고 다시 읽으면 같아야 한다.** 찍는 쪽과 읽는 쪽이 어긋나면
// 관문이 매번 붉어지고, 그러면 기준선을 지우는 것으로 끝난다.
func TestBaselineRoundTrips(t *testing.T) {
	want := map[key]int{}
	want[key{"엠대시", "docs/design.md"}] = 55
	want[key{"조용히", "README.md"}] = 1
	p := filepath.Join(t.TempDir(), "baseline.tsv")
	if err := writeBaseline(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := readBaseline(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("항목 수가 다르다: %d → %d", len(want), len(got))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%v: %d → %d", k, v, got[k])
		}
	}
	grown, shrunk := compare(got, want)
	if len(grown) != 0 || len(shrunk) != 0 {
		t.Errorf("찍고 읽었더니 달라졌다: %v %v", grown, shrunk)
	}
}

// **규칙표가 실제로 읽힌다.** 탭이 하나 빠지거나 정규식이 깨지면 관문이 아예
// 서지 않는데, 그 사실을 빌드가 알려 주지 않는다.
func TestShippedRulesLoad(t *testing.T) {
	rs, err := loadRules(filepath.Join("..", "..", "tools", "checkprose", rulesName))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 5 {
		t.Fatalf("규칙이 너무 적다: %d개", len(rs))
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if seen[r.name] {
			t.Errorf("이름이 겹친다: %s", r.name)
		}
		seen[r.name] = true
		if r.fix == "" {
			t.Errorf("%s: 무엇으로 바꿀지 적혀 있지 않다", r.name)
		}
	}
	if !seen["엠대시"] {
		t.Error("엠대시 규칙이 없다")
	}
}

// **덮는 자리가 목록에 적혀 있다.** 도구 메시지가 든 Go 파일과 HTML 페이지는 규약으로
// 두면 새 파일이 슬그머니 관문 밖이 된다. 목록에 있는 파일이 실제로 있는지 잰다.
func TestListedScreenFilesExist(t *testing.T) {
	files, err := loadWords(filepath.Join("..", "..", "tools", "checkprose", filesName))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("files.txt 가 비어 있다")
	}
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join("..", "..", rel)); err != nil {
			t.Errorf("목록에 있는데 없는 파일이다: %s (%v)", rel, err)
		}
		if !strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, ".html") {
			t.Errorf("Go·HTML 만 적을 수 있다: %s", rel)
		}
	}
}

// **잘못 잡는 말을 덮어도 바이트 수가 그대로다.** 여기가 어긋나면 그 뒤의 모든
// 줄 번호가 밀린다.
func TestHideOverlapKeepsLength(t *testing.T) {
	useShippedOverlap(t)
	for _, s := range []string{"헷갈리는", "헛갈리기 쉽다", "갈리면"} {
		if got := len(hideOverlap(s)); got != len(s) {
			t.Errorf("%q: 바이트 수가 달라졌다 %d → %d", s, len(s), got)
		}
	}
	if regexp.MustCompile("갈리는").MatchString(hideOverlap("헷갈리는")) {
		t.Error("헷갈리는을 덮지 못했다")
	}
}

// **한국어가 없는 줄은 보지 않는다.**
//
// 지침은 한국어를 명확하게 쓰라는 것이지 외국어를 고치라는 것이 아니다(「동작 범위」 1항).
// 소개 페이지가 KO 와 EN 을 나란히 적는 자리라, 이 선이 없으면 영어 문장의 엠대시까지
// 세어 고칠 수 없는 것을 요구하게 된다.
func TestLinesWithoutKoreanAreNotCounted(t *testing.T) {
	rs := mustRules(t, "엠대시\t—\t콜론으로")
	src := []byte("Saved to the session file — not finalized yet\n세션 파일에 저장했습니다 — 아직입니다\n")
	got := hitsOf("a.md", src, src, rs)
	if len(got) != 1 {
		t.Fatalf("영어 줄까지 셌다: %d건 %v", len(got), got)
	}
	if got[0].line != 2 {
		t.Errorf("줄 번호가 어긋났다: %d", got[0].line)
	}
}

// **한 줄에 KO 와 EN 이 나란히 있어도 한국어만 잰다.**
//
// 두 말을 `T{KO: …, EN: …}` 처럼 한 줄에 적는 Go 파일이 있을 수 있다. 줄 단위로만 보면 영어
// 문장의 엠대시까지 세는데, 영어에서 그것은 맞는 문장부호다. 그래서 **문자열 하나 단위로**
// 한국어가 들었는지 본다.
func TestEnglishStringOnTheSameLineIsNotCounted(t *testing.T) {
	rs := mustRules(t, "엠대시\t—\t콜론으로")
	dir := t.TempDir()
	p := filepath.Join(dir, "text.go")
	src := "package ui\n\nvar t = T{KO: \"고전(양자 취약)\", EN: \"classical — quantum-vulnerable\"}\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, masked, err := maskGo(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitsOf("text.go", orig, masked, rs); len(got) != 0 {
		t.Fatalf("영어 문자열까지 셌다: %v", got)
	}
}

// **코드 뒤에 띄운 조사는 잡고, 붙인 조사는 잡지 않는다.** 인라인 코드를 공백으로
// 지우면 「`x` 에」와 「`x`에」가 똑같이 「   에」가 되어 둘을 가르지 못한다. 그래서 같은 길이의
// 채움 글자로 가린다. HTML 의 `<code>` 도 같다.
func TestSpacedParticleAfterCodeIsCaughtButAttachedIsNot(t *testing.T) {
	rs := mustRules(t, "띄운 조사\t(?m)(^|[ \\t])(가|은|는|을|를|에|의)([ \\t.,)]|$)\t붙여 쓴다")
	src := []byte("`go.mod` 가 판을 고정한다. `go.mod`가 판을 고정한다. pqcota 의 것. pqcota의 것.\n")
	got := hitsOf("a.md", src, maskMarkdown(src), rs)
	if len(got) != 2 {
		t.Fatalf("띄운 조사 둘만 잡혀야 한다: %d건 %v", len(got), got)
	}
	html := []byte("<p><code>go.mod</code>가 판을 고정한다. <code>go.mod</code> 가 판을 고정한다.</p>\n")
	got = hitsOf("a.html", html, maskHTML(html), rs)
	if len(got) != 1 {
		t.Fatalf("HTML 에서 띄운 조사 하나만 잡혀야 한다: %d건 %v", len(got), got)
	}
}

// **함께 나가는 rules.tsv 의 띄운 조사 규칙**을 실제 문장으로 잰다. 「에만」·「나」·
// 「까지만」·「로만」·「뿐」·「라」·「라고」·「였습니다」·「여야」처럼 뒤늦게 더한 조사가 잡히는지,
// 조사 뒤에 `**`·`<`·「」」가 와도 잡히는지, 그리고 「할 뿐」·「쓰다 만」처럼 낱말로 서는
// 「뿐」·「만」을 잘못 잡지 않는지. 규칙을 테스트 안에 따로 적으면 파일이 바뀌어도
// 케이스가 통과한다.
func TestShippedSpacedParticleRule(t *testing.T) {
	all, err := loadRules(filepath.Join("..", "..", "tools", "checkprose", rulesName))
	if err != nil {
		t.Fatal(err)
	}
	var rs []rule
	for _, r := range all {
		if r.name == "띄운 조사" {
			rs = append(rs, r)
		}
	}
	if len(rs) != 1 {
		t.Fatalf("띄운 조사 규칙이 하나여야 한다: %d", len(rs))
	}
	caught := []string{
		"훅은 L3 에만 묻는다.",
		"방금 나타난 UNDECLARED 나 방금 넣은 것.",
		"`pqcota-ingest` 까지만 돌린다.",
		"한 role 로만 붙는다.",
		"`sshd`와 `python` 뿐 아니라",
		"§ 로만 적으면",
		"상류의 id는 `sha256(노드|이름)` 라 자산이 같으면",
		"조치 종류가 비면 `PROVIDER_INJECT` 였습니다.",
		"이 문서 첫머리가 **「링크입니다」** 라고 약속한다.",
		"**Linux 에서만** 됩니다.",
		"「그대로」 였고, 그때는",
	}
	for _, s := range caught {
		src := []byte(s + "\n")
		if got := hitsOf("a.md", src, maskMarkdown(src), rs); len(got) != 1 {
			t.Errorf("잡혀야 한다: %q → %d건 %v", s, len(got), got)
		}
	}
	clean := []string{
		"훅은 L3에만 묻는다. UNDECLARED나 `exclude`가. `pqcota-ingest`까지만. role로만. `python`뿐 아니라",
		"판정 대상을 구조화할 뿐, 확정은 사람이 한다.",
		"쓰다 만 파일을 남기지 않는다. 채우다 만 것을",
		"이름표일 뿐 관측 대상이 아니다.",
		"머리에서 「관측 2」라 하고, `PROVIDER_INJECT`였습니다. **Linux에서만** 됩니다. `libcrypto`여야 하고.",
	}
	for _, s := range clean {
		src := []byte(s + "\n")
		if got := hitsOf("a.md", src, maskMarkdown(src), rs); len(got) != 0 {
			t.Errorf("잡히면 안 된다: %q → %v", s, got)
		}
	}
	html := []byte("<p>오른쪽이 없으면 <em>판정이 없을</em> 뿐 관측은 된다. <code>x</code> 뿐 아니라</p>\n")
	if got := hitsOf("a.html", html, maskHTML(html), rs); len(got) != 1 {
		t.Errorf("HTML 태그 뒤의 「뿐」은 두고 코드 뒤만 잡아야 한다: %d건 %v", len(got), got)
	}
	html = []byte("<p><b>자산이 통째로 UNDECLARED 로</b> 올라온다. <code>libcrypto</code> 여야 하고, <b>UNDECLARED로</b> 구분된다.</p>\n")
	if got := hitsOf("a.html", html, maskHTML(html), rs); len(got) != 2 {
		t.Errorf("태그가 바로 뒤에 와도 띄운 조사 둘을 잡고 붙인 것은 두어야 한다: %d건 %v", len(got), got)
	}
}

// **알림표는 관문이 아니다.** notices.tsv 가 읽히고 실제 문장에 걸리되, 표 행과
// 제목은 비켜 가며, 관문 규칙과 섞이지 않는다. 막는 규칙으로 두면 제목·표의 정당한 구분
// 기호까지 예외 목록에 쌓이므로 후보로만 알린다.
func TestShippedNoticesFlagCandidatesWithoutGating(t *testing.T) {
	ns, err := loadRules(filepath.Join("..", "..", "tools", "checkprose", noticesName))
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) == 0 {
		t.Fatal("notices.tsv 가 비어 있다")
	}
	src := []byte(strings.Join([]string{
		"**노드 수**로 셉니다 - 관측하거나 적용하는 대상입니다.",
		"| 구독 - 관측/인벤토리 | 0.86억 |",
		"## 5. 데이터 - 무엇을 내보내나",
		"노드 수로 셉니다. 노드는 관측 대상입니다.",
	}, "\n") + "\n")
	got := hitsOf("a.md", src, maskMarkdown(src), ns)
	if len(got) != 1 || got[0].line != 1 {
		t.Fatalf("본문의 띄운 붙임표 하나만 걸려야 한다(표·제목은 제외): %d건 %v", len(got), got)
	}
	rs, err := loadRules(filepath.Join("..", "..", "tools", "checkprose", rulesName))
	if err != nil {
		t.Fatal(err)
	}
	if gate := hitsOf("a.md", src, maskMarkdown(src), rs); len(gate) != 0 {
		t.Fatalf("알림표의 문장이 관문 규칙에 걸리면 안 된다: %v", gate)
	}
}

// **실제 실행 경로에서 알림은 통과하고 기준선에 섞이지 않는다.** IC-K16 은 규칙
// 분리를 재지만 명령의 종료 코드와 기준선 파일은 재지 않는다. 알림만 있는 입력으로 -baseline
// 을 찍으면 기준선이 비고, 관문을 돌리면 0 으로 끝나며, 관문 규칙에 걸리는 줄을 더하면 1 이 된다.
func TestNoticesPassTheGateAndStayOutOfTheBaseline(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join(dir, "tools", "checkprose"), 0o755); err != nil {
		t.Fatal(err)
	}
	put := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := filepath.Join("tools", "checkprose")
	put(filepath.Join(cfg, rulesName), "엠대시\t—\t콜론으로 푼다\n")
	put(filepath.Join(cfg, noticesName), "띄운 붙임표\t(?m)^[^|#\\n]*[가-힣][ \\t]+-[ \\t]+[가-힣]\t마침표나 콜론으로 푼다\n")
	put(filepath.Join(cfg, overlapName), "# 없음\n")
	put(filepath.Join(cfg, filesName), "# 없음\n")
	put("a.md", "노드 수로 셉니다 - 관측 대상입니다.\n")

	if code := run(cfg, false, true); code != 0 {
		t.Fatalf("알림만 있는 입력의 -baseline 이 실패했다: %d", code)
	}
	b, err := os.ReadFile(filepath.Join(cfg, baselineName))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			t.Fatalf("알림이 기준선에 들어갔다: %q", line)
		}
	}
	if code := run(cfg, false, false); code != 0 {
		t.Fatalf("알림만 있는 입력이 관문을 막았다: %d", code)
	}
	put("a.md", "노드 수로 셉니다 - 관측 대상입니다.\n관측 — 실행 중인 것을 본다.\n")
	if code := run(cfg, false, false); code != 1 {
		t.Fatalf("관문 규칙에 걸리는 줄을 더했는데 막지 않았다: %d", code)
	}
}

// **HTML 주석은 보지 않는다.** 시연영상 카드의 주석에 촬영 메모가 한국어로 적혀 있고, 그것은
// 코드 주석과 같은 자리다. 주석을 덮어도 줄 수는 그대로여야 뒤 줄 번호가 맞는다.
func TestHTMLCommentsAreNotCounted(t *testing.T) {
	rs := mustRules(t, "엠대시\t—\t콜론으로")
	html := []byte("<!-- 컷 1 — 훅\n  두 줄짜리 주석 — -->\n<p>본문 — 여기는 센다</p>\n")
	masked := maskHTML(html)
	if strings.Count(string(masked), "\n") != strings.Count(string(html), "\n") {
		t.Fatal("주석을 덮으면서 줄 수가 달라졌다")
	}
	got := hitsOf("a.html", html, masked, rs)
	if len(got) != 1 || got[0].line != 3 {
		t.Fatalf("주석 밖의 엠대시 하나만 3행에서 잡혀야 한다: %d건 %v", len(got), got)
	}
}

// **다시 돌리는 안내는 명령을 지어내지 않고 인자만 적는다.** 이 도구는 `go run ./tools/checkprose`
// 로도 `go run <모듈>@<판>` 으로도 돌아 실행 명령을 알 수 없고, 실행 파일 이름이 있다고 보장할
// 수도 없다. 기본 디렉터리면 플래그만, 지정했으면 -dir 을 보존하고, 공백이 든 경로는 인용한다.
func TestRerunHintNamesArgumentsOnly(t *testing.T) {
	cases := map[string]string{
		"":                 "rerun the same command with: -list",
		"tools/checkprose": "rerun the same command with: -list",
		"cfg":              "rerun the same command with: -dir \"cfg\" -list",
		"my config/prose":  "rerun the same command with: -dir \"my config/prose\" -list",
	}
	for dir, want := range cases {
		if got := rerun(dir, "-list"); got != want {
			t.Errorf("rerun(%q): got %q, want %q", dir, got, want)
		}
		if strings.Contains(rerun(dir, "-list"), "checkprose -") || strings.Contains(rerun(dir, "-list"), "go run") {
			t.Errorf("rerun(%q) names a command: %q", dir, rerun(dir, "-list"))
		}
	}
}
