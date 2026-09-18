// Command checkprose — 문서와 도구 출력의 **한국어 문체**를 본다.
//
// 규칙은 하나다: **한 번 걷어낸 말이 다시 들어오지 않는다.**
//
// 왜 관문인가. 2026-08-25에 엠대시를 `.md`에서 걷어냈는데 `.html` 화면 문구에는 그대로
// 남았고, 「조용히」(silently)는 그 뒤로도 쉰 줄 넘게 쌓였다. 눈으로 지킨 규칙은 남지 않고
// **기계로 막은 규칙만** 남는다.
//
// **기준선을 둔다.** 지금 있는 것을 한꺼번에 막으면 관문이 처음부터 붉어서 아무도 켜지
// 않는다. 그래서 파일마다 지금 개수를 적어 두고 **늘면 막는다.** 줄여도 막고 새 기준선을
// 찍어 준다: 고쳐 놓고 기준선을 안 내리면 그 자리가 도로 채워져도 알 수 없다.
//
// **한국어가 없는 줄은 보지 않는다.** 지침은 한국어를 명확하게 쓰라는 것이지 외국어를
// 고치라는 것이 아니다. 같은 이유로 영문 짝(`*.en.md`)은 파일째 보지 않는다.
//
// **코드는 보지 않는다.** 변수명·주석·커밋·로그처럼 코드에 속하는 텍스트는 프로젝트
// 관례를 따르는 자리다. 그래서 마크다운은 코드 블록과 인라인 코드를 덮고 나서 보며,
// Go는 go/ast로 **문자열 리터럴만** 본다(주석은 한국어다). HTML은 code·pre·script·
// style과 주석 안을 덮는다.
//
// **규칙은 rules.tsv에, 잘못 잡는 말은 overlap.txt에 있다.** 두 목록 모두 지적받을 때마다
// 늘어나므로 코드 밖에 있어야 한다.
//
// **알림표 notices.tsv는 관문이 아니다.** 맞는 용법이 섞여 있어 기계가 가르지 못하는 것
// (제목·표에서는 구분 기호인 띄운 붙임표 같은 것)은 막지 않고 후보로만 알린다. 걸려도
// 통과하고 기준선에도 넣지 않는다. 막는 규칙으로 두면 예외 목록이 쌓이고, 예외가 쌓이면
// 진짜 위반도 함께 묻힌다.
//
// **출처와 권리.** 이 도구는 https://github.com/sntsoftgit/pqcaton의 tools/checkprose(커밋
// 118970a~15ac503, 2026-08-25~09-16, 그 리포는 BUSL-1.1)에서 시작했다. 권리자 (주)에스앤티소프트
// (SNT Soft Co., Ltd.)가 2026-09-17에 이것을 이 리포의 LICENSE(Apache-2.0)로 제공하기로 결정했고,
// **그때부터 이 리포가 원본이다.** 다른 리포는 코드를 복사하지 않고 판을 지정해 돌린다.
//
// **설정은 전부 한 디렉터리(-dir, 기본 tools/checkprose)에 있고 코드는 리포를 가리지 않는다.**
// rules.tsv(규칙) · notices.tsv(알림) · overlap.txt(잘못 잡는 말) · files.txt(마크다운 밖에서
// 볼 Go·HTML 파일) · baseline.tsv(기준선). 다른 리포는 이 코드를 복사하지 않고 자기 설정
// 디렉터리를 두고 `go run github.com/randyinthedev-hash/pqcota/tools/checkprose@<판>`으로 돌린다.
//
// usage:
//
//	go run ./tools/checkprose             # 관문
//	go run ./tools/checkprose -list       # 걸린 자리를 줄 번호까지(알림 포함)
//	go run ./tools/checkprose -baseline   # 기준선을 다시 찍는다
//	go run ./tools/checkprose -dir <설정 디렉터리>
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 설정 파일 이름. 디렉터리는 -dir로 받는다(기본 tools/checkprose).
const (
	rulesName    = "rules.tsv"
	noticesName  = "notices.tsv"
	baselineName = "baseline.tsv"
	overlapName  = "overlap.txt"
	filesName    = "files.txt"
)

// overlap — 규칙이 잘못 잡는 말. 재기 전에 같은 길이로 덮는다. 「헷갈리다」의 "갈리"가
// 「갈리다」 규칙에 걸리는 것이 실제로 나온 자리라, 뒤보기 없는 RE2에서는 이 편이 낫다.
//
// 말 자체는 overlap.txt에 있다. 지적받을 때마다 늘어나는 목록이라 코드 밖에 둔다.
var overlap []string

type rule struct {
	name string
	re   *regexp.Regexp
	fix  string
}

type hit struct {
	rule string
	file string
	line int
	text string
}

func main() {
	dir := flag.String("dir", "tools/checkprose", "directory holding rules.tsv, notices.tsv, overlap.txt, files.txt and baseline.tsv")
	list := flag.Bool("list", false, "print every hit with its line")
	write := flag.Bool("baseline", false, "rewrite the baseline file")
	flag.Parse()
	os.Exit(run(*dir, *list, *write))
}

// run — 관문 한 번. 종료 코드를 돌려주고 os.Exit은 main이 한다: 실제 실행 경로(알림만
// 있는 입력이 통과하는지, 기준선에 알림이 섞이지 않는지)를 테스트가 그대로 밟기 위해서다.
func run(dir string, list, write bool) int {
	rulesFile := filepath.Join(dir, rulesName)
	baselineFile := filepath.Join(dir, baselineName)
	rules, err := loadRules(rulesFile)
	if err != nil {
		return failed(err)
	}
	overlap, err = loadWords(filepath.Join(dir, overlapName))
	if err != nil {
		return failed(err)
	}
	extra, err := loadWords(filepath.Join(dir, filesName))
	if err != nil {
		return failed(err)
	}
	hits, err := scan(".", extra, rules)
	if err != nil {
		return failed(err)
	}
	notices, err := loadRules(filepath.Join(dir, noticesName))
	if err != nil {
		return failed(err)
	}
	noted, err := scan(".", extra, notices)
	if err != nil {
		return failed(err)
	}
	if list {
		printList(hits)
		printNotices(noted)
	}
	counts := tally(hits)

	if write {
		if err := writeBaseline(baselineFile, counts); err != nil {
			return failed(err)
		}
		fmt.Printf("✓ baseline rewritten: %d entries, %d hits\n", len(counts), len(hits))
		printNoticeCount(dir, noted)
		return 0
	}

	base, err := readBaseline(baselineFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ no usable baseline:", err)
		fmt.Fprintln(os.Stderr, " ", rerun(dir, "-baseline"))
		return 1
	}
	grown, shrunk := compare(base, counts)
	if len(grown) > 0 {
		fmt.Fprintln(os.Stderr, "✗ prose gate: these grew past the baseline")
		for _, l := range grown {
			fmt.Fprintln(os.Stderr, "   ", l)
		}
		fmt.Fprintln(os.Stderr, "\nTo see them,", rerun(dir, "-list"))
		fmt.Fprintln(os.Stderr, "Each rule in", strconv.Quote(rulesFile), "says what to write instead.")
		return 1
	}
	if len(shrunk) > 0 {
		fmt.Fprintln(os.Stderr, "✗ prose gate: the baseline is stale — these went down")
		for _, l := range shrunk {
			fmt.Fprintln(os.Stderr, "   ", l)
		}
		fmt.Fprintln(os.Stderr, "\nTo lock the win in,", rerun(dir, "-baseline"))
		return 1
	}
	fmt.Printf("✓ prose check passed (%d hits, all at the baseline)\n", len(hits))
	printNoticeCount(dir, noted)
	return 0
}

func failed(err error) int {
	fmt.Fprintln(os.Stderr, "✗ checkprose:", err)
	return 1
}

// rerun — 안내문에 적는 「다시 돌리는 법」. 이 도구는 자기 리포에서 `go run ./tools/checkprose`
// 로도, 다른 리포에서 `go run <모듈 경로>@<판>`으로도 돌므로 실행한 명령을 알 수 없고, 실행
// 파일 이름을 지어내지도 않는다. **같은 명령에 줄 인자만** 적되, 사용자가 준 -dir은 잃지 않는다.
// 경로에 공백이 있을 수 있어 인용해 보인다.
func rerun(dir, flag string) string {
	args := flag
	if dir != "" && dir != "tools/checkprose" {
		args = "-dir " + strconv.Quote(dir) + " " + flag
	}
	return "rerun the same command with: " + args
}

// ── 규칙 ───────────────────────────────────────────────────────────────────

func loadRules(path string) ([]rule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []rule
	for i, line := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("%s:%d: want 3 tab-separated fields, got %d", path, i+1, len(f))
		}
		re, err := regexp.Compile(f[1])
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
		}
		out = append(out, rule{name: f[0], re: re, fix: f[2]})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no rules", path)
	}
	return out, nil
}

// loadWords — 한 줄에 하나씩 적은 말 목록. 빈 줄과 #로 시작하는 줄은 넘긴다.
func loadWords(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// ── 훑기 ───────────────────────────────────────────────────────────────────

// scan — root 아래의 마크다운 전부와, extra에 적힌 Go·HTML 파일을 잰다.
func scan(root string, extra []string, rules []rule) ([]hit, error) {
	var out []hit
	seen := map[string]bool{}

	// 원문도 함께 넘긴다. 덮은 쪽으로 재고, **보여 줄 때는 원문 줄을 보여 준다**:
	// 덮인 줄을 그대로 찍으면 어느 문장인지 알아볼 수 없다. 덮기가 바이트 수를
	// 그대로 두므로 원문과 덮은 것의 자리가 어긋나지 않는다.
	add := func(rel string, orig, masked []byte) {
		if seen[rel] {
			return
		}
		seen[rel] = true
		out = append(out, match(rel, orig, masked, rules)...)
	}

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// `_workspace`는 첨삭 도구가 만드는 작업 폴더다. **고칠 문장을 그대로
			// 인용해 둔 파일**이라 걸리는 것이 당연하고, 날짜별로 새 폴더가 생기니
			// 기준선에 넣으면 날마다 썩는다. .gitignore도 이미 이 폴더를 뺀다.
			if n := d.Name(); n == ".git" || n == "node_modules" || n == "testdata" || n == "_workspace" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		// 영문 짝은 보지 않는다. 한국어 규칙을 영어 문서에 걸 이유가 없고, 영문 안에 인용된
		// 한국어 낱말(「축」 같은 것)까지 세면 그 줄이 오탐이 된다.
		if !strings.HasSuffix(rel, ".md") || strings.HasSuffix(rel, ".en.md") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		add(rel, b, maskMarkdown(b))
		return nil
	})
	if err != nil {
		return nil, err
	}

	// files.txt — 마크다운 밖에서 사람이 읽는 한국어가 있는 파일. **경로로 적는다**: 규약으로
	// 두면 새 파일이 슬그머니 들어오거나 빠진다. 확장자로 덮는 법을 고른다.
	for _, rel := range extra {
		switch {
		case strings.HasSuffix(rel, ".go"):
			orig, masked, err := maskGo(filepath.Join(root, rel))
			if err != nil {
				return nil, err
			}
			add(rel, orig, masked)
		case strings.HasSuffix(rel, ".html"):
			b, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				return nil, err
			}
			add(rel, b, maskHTML(b))
		default:
			return nil, fmt.Errorf("%s: only .go and .html can be listed in %s", rel, filesName)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].file != out[j].file {
			return out[i].file < out[j].file
		}
		if out[i].line != out[j].line {
			return out[i].line < out[j].line
		}
		return out[i].rule < out[j].rule
	})
	return out, nil
}

func match(rel string, orig, masked []byte, rules []rule) []hit {
	s := hideOverlap(string(masked))
	var out []hit
	for _, r := range rules {
		for _, loc := range r.re.FindAllStringIndex(s, -1) {
			// **한국어가 없는 줄은 보지 않는다.** 지침은 한국어를 명확하게 쓰라는 것이지
			// 외국어를 고치라는 것이 아니다(「동작 범위」 1항). 소개 페이지는 KO와 EN을
			// 나란히 적는 자리라, 이 선이 없으면 EN 문장의 엠대시까지 세게 된다.
			if !hasHangul(lineAt(s, loc[0])) {
				continue
			}
			out = append(out, hit{
				rule: r.name,
				file: rel,
				line: 1 + strings.Count(s[:loc[0]], "\n"),
				text: excerpt(string(orig), loc[0]),
			})
		}
	}
	return out
}

// lineAt — 그 자리가 든 한 줄.
func lineAt(s string, off int) string {
	start := strings.LastIndexByte(s[:off], '\n') + 1
	end := strings.IndexByte(s[off:], '\n')
	if end < 0 {
		return s[start:]
	}
	return s[start : off+end]
}

func hasHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

// hideOverlap — 잘못 잡는 말을 **같은 바이트 수로** 덮는다. 자리가 밀리면 줄 번호가
// 어긋나므로 지우지 않는다.
func hideOverlap(s string) string {
	for _, w := range overlap {
		s = strings.ReplaceAll(s, w, strings.Repeat("◌", len([]rune(w))))
	}
	return s
}

// excerpt — 걸린 자리가 어느 문장인지 알아볼 만큼만 그 줄에서 떼어 온다.
func excerpt(s string, off int) string {
	start := strings.LastIndexByte(s[:off], '\n') + 1
	end := strings.IndexByte(s[off:], '\n')
	if end < 0 {
		end = len(s)
	} else {
		end += off
	}
	line := strings.TrimSpace(s[start:end])
	r := []rune(line)
	if len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return line
}

// ── 덮기 ───────────────────────────────────────────────────────────────────
//
// 덮는 자리는 **같은 길이의 채움 글자로 바꾼다**(fill). 줄바꿈은 그대로 두어야 줄 번호가 맞는다.

var (
	fence       = regexp.MustCompile("(?m)^\\s*(```|~~~)")
	inlineMD    = regexp.MustCompile("`[^`\n]*`")
	htmlBlock   = regexp.MustCompile(htmlBlockPattern())
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// htmlBlockPattern — 여는 태그와 닫는 태그를 짝지어야 하는데 RE2에는 역참조가 없다.
// 그래서 태그마다 따로 적어 이어 붙인다.
func htmlBlockPattern() string {
	parts := make([]string, 0, 4)
	for _, t := range []string{"code", "pre", "script", "style"} {
		parts = append(parts, `<`+t+`\b[^>]*>.*?</\s*`+t+`\s*>`)
	}
	return `(?is)` + strings.Join(parts, "|")
}

func maskMarkdown(b []byte) []byte {
	lines := strings.Split(string(b), "\n")
	in := false
	for i, l := range lines {
		if fence.MatchString(l) {
			in = !in
			lines[i] = blank(l)
			continue
		}
		if in {
			lines[i] = blank(l)
			continue
		}
		lines[i] = inlineMD.ReplaceAllStringFunc(l, fill)
	}
	return []byte(strings.Join(lines, "\n"))
}

// fill — 인라인 코드를 **공백이 아니라 채움 글자**로 가린다. 길이는 그대로라 열 위치가 맞고,
// 규칙은 그 안을 보지 못한다. 공백으로 지우면 「`x` 에」와 「`x`에」가 똑같이 「   에」가 되어
// 띄운 조사 규칙이 둘을 가르지 못한다. 채움 글자는 한글도 조사도 아닌 것이라 다른 규칙에는
// 걸리지 않는다.
func fill(s string) string {
	return strings.Repeat("x", len(s))
}

func maskHTML(b []byte) []byte {
	// 코드 블록은 줄바꿈만 남기고 채움 글자로 가린다(fill과 같은 이유 - 공백으로 지우면
	// 「<code>x</code>가」가 띄운 조사로 읽힌다). HTML 주석도 덮는다: 코드 주석과 같은 자리라
	// 프로젝트 관례를 따르고, 화면에 보이지 않는다.
	s := htmlBlock.ReplaceAllStringFunc(string(b), fillKeepNewlines)
	return []byte(htmlComment.ReplaceAllStringFunc(s, fillKeepNewlines))
}

func fillKeepNewlines(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c != '\n' {
			out[i] = 'x'
		}
	}
	return string(out)
}

// maskGo — 문자열 리터럴만 남기고 나머지를 덮는다. 주석은 한국어이므로 보지 않는다.
// 원문과 덮은 것을 함께 돌려준다.
func maskGo(path string) (orig, masked []byte, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, b, 0)
	if err != nil {
		return nil, nil, err
	}
	out := []byte(keepNewlines(string(b)))
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		// **한국어가 든 문자열만 남긴다.** 카탈로그는 KO와 EN을 나란히 적으므로, 줄 단위로
		// 보면 같은 줄의 영어 문장까지 재게 된다. 영어의 엠대시는 영어에서 맞는 문장부호다.
		if !hasHangul(lit.Value) {
			return true
		}
		from := fset.Position(lit.Pos()).Offset
		to := fset.Position(lit.End()).Offset
		if from < 0 || to > len(out) {
			return true
		}
		copy(out[from:to], b[from:to])
		return true
	})
	return b, out, nil
}

func blank(s string) string {
	return strings.Repeat(" ", len(s))
}

func keepNewlines(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c != '\n' {
			out[i] = ' '
		}
	}
	return string(out)
}

// ── 기준선 ─────────────────────────────────────────────────────────────────

type key struct {
	rule string
	file string
}

func tally(hits []hit) map[key]int {
	out := map[key]int{}
	for _, h := range hits {
		out[key{h.rule, h.file}]++
	}
	return out
}

func writeBaseline(path string, counts map[key]int) error {
	var b strings.Builder
	b.WriteString("# checkprose baseline. count<TAB>rule<TAB>file\n")
	fmt.Fprintf(&b, "# To rewrite, %s\n", rerun(filepath.Dir(path), "-baseline"))
	for _, k := range sortedKeys(counts) {
		fmt.Fprintf(&b, "%d\t%s\t%s\n", counts[k], k.rule, k.file)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func readBaseline(path string) (map[key]int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[key]int{}
	for i, line := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("%s:%d: want 3 tab-separated fields, got %d", path, i+1, len(f))
		}
		n, err := strconv.Atoi(f[0])
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
		}
		out[key{f[1], f[2]}] = n
	}
	return out, nil
}

// compare — 늘어난 것과 줄어든 것을 따로 돌려준다. 둘 다 관문을 막는다: 늘면 새로 들어온
// 것이고, 줄면 기준선이 낡은 것이다.
func compare(base, now map[key]int) (grown, shrunk []string) {
	all := map[key]bool{}
	for k := range base {
		all[k] = true
	}
	for k := range now {
		all[k] = true
	}
	var keys []key
	for k := range all {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].file != keys[j].file {
			return keys[i].file < keys[j].file
		}
		return keys[i].rule < keys[j].rule
	})
	for _, k := range keys {
		b, n := base[k], now[k]
		switch {
		case n > b:
			grown = append(grown, fmt.Sprintf("%s  %s  %d → %d  (+%d)", k.file, k.rule, b, n, n-b))
		case n < b:
			shrunk = append(shrunk, fmt.Sprintf("%s  %s  %d → %d  (-%d)", k.file, k.rule, b, n, b-n))
		}
	}
	return grown, shrunk
}

func sortedKeys(counts map[key]int) []key {
	var out []key
	for k := range counts {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].file != out[j].file {
			return out[i].file < out[j].file
		}
		return out[i].rule < out[j].rule
	})
	return out
}

func printList(hits []hit) {
	for _, h := range hits {
		fmt.Printf("%s:%d\t%s\t%s\n", h.file, h.line, h.rule, h.text)
	}
	fmt.Printf("%d hits\n", len(hits))
}

// printNotices — 알림표에 걸린 자리. 관문 판정과 섞이지 않게 따로 찍는다.
func printNotices(noted []hit) {
	for _, h := range noted {
		fmt.Printf("%s:%d\tnotice: %s\t%s\n", h.file, h.line, h.rule, h.text)
	}
	fmt.Printf("%d notices (not gated)\n", len(noted))
}

// printNoticeCount — 관문 결과 뒤에 알림 건수만 한 줄. 0 이면 아무것도 찍지 않는다.
func printNoticeCount(dir string, noted []hit) {
	if len(noted) == 0 {
		return
	}
	fmt.Printf("  %d notices to review (not gated). To see them, %s\n", len(noted), rerun(dir, "-list"))
}
