// Command checkgates — **규칙은 있는데 제품이 부르지 않는 게이트**를 막는다.
//
// 이 리포는 보장을 함수로 적어 둔다. `provisioning.Executable`은 확정 계획이 실행 근거인지 보고,
// `sign.VerifyFrom`은 서명이 그 collector의 것인지 본다. 그런데 **그 함수를 제품 경로가 부르지
// 않으면 보장이 아니다.** 실제로 그랬다: `pqcota-provision`이 `Executable`을 부르지 않고 상태만
// 인라인으로 비교하는 동안, 승인 서명이 빈 FINALIZED 계획이 플레이북을 받아 갔다. 규칙에는 테스트가
// 있었고 문서는 그 규칙을 약속했으며 게이트는 전부 초록이었다.
//
// **테스트는 규칙이 옳은지 보지, 그 규칙이 쓰이는지 보지 않는다.** 그 자리를 여기서 막는다.
//
// # 표시
//
// 등록은 함수 자리에서 한다. 별도 목록 파일을 두면 목록과 대상이 떨어져 어긋난다(이 리포에서
// 실제로 여러 번 겪었다). 함수를 옮기거나 지우면 표시도 따라간다.
//
//	// GATE: 배선 필수
//	func Executable(...) error
//
//	// GATE: 보류 — 왜 아직 안 이었는지, 어디에 적혀 있는지
//	func VerifyFrom(...) bool
//
// `보류`는 통과시키되 **그 사실을 화면에 낸다.** 면제가 통과와 같은 모양이면 미룬 것이 시간이
// 지나며 보이지 않게 된다(§2.6 제외는 부재가 아니다).
//
// # 무엇을 보지 못하나
//
// **메서드는 대상이 아니다.** `(*scope.Master).ClassifyObserved`처럼 리시버에 붙은 것은
// `m.ClassifyObserved(...)`로 불려 좌변이 변수다. 그것을 가리려면 타입까지 풀어야 하고, 그 해석은
// 이 검사기가 감당하는 범위 밖이다. 그런 것은 표시를 붙이지 않고 검토 중인 설계가 계속 들고 간다.
// 못 보는 것을 안 보는 척하지 않는다.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// marker — 이 낱말이 doc 주석에 있으면 등록된다.
const marker = "GATE:"

// pending — 배선을 미룬 것. 통과시키되 고지한다.
const pending = "보류"

// gate — 등록된 보장 함수 하나.
type gate struct {
	Name    string // 함수 이름
	Pkg     string // 선언된 패키지 이름 (호출부를 가릴 때 쓴다)
	File    string // 선언 파일 (여기서의 호출은 세지 않는다)
	Note    string // GATE: 뒤에 적힌 말
	Pending bool
}

func main() {
	files, err := goFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	miss, notes, err := check(files)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, n := range notes {
		fmt.Fprintln(os.Stderr, "· "+n)
	}
	if len(miss) > 0 {
		fmt.Println("✗ 규칙은 있는데 제품 경로가 부르지 않는다 — 배선하거나, 왜 미루는지 `GATE: 보류`로 적을 것:")
		for _, m := range miss {
			fmt.Println("    " + m)
		}
		fmt.Println()
		fmt.Println("gates check failed — fix the locations above and run `make check-gates` again.")
		os.Exit(1)
	}
	fmt.Printf("✓ gates check passed (%d개 등록 · 보류 %d개)\n", len(notes)+countWired(files), len(notes))
}

// goFiles — 추적 중인 Go 파일. **testdata는 뺀다.** 이 검사기 자신의 fixture가 들어 있어,
// 빼지 않으면 가짜 표시가 진짜로 잡혀 게이트가 자기 fixture 때문에 깨진다.
func goFiles() ([]string, error) {
	out, err := exec.Command("git", "-c", "core.quotePath=off", "ls-files", "*.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var keep []string
	for _, f := range strings.Fields(string(out)) {
		if strings.HasPrefix(f, "gen/") || hasDir(f, "testdata") {
			continue
		}
		keep = append(keep, f)
	}
	return keep, nil
}

func hasDir(path, dir string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		if seg == dir {
			return true
		}
	}
	return false
}

// check — 등록된 게이트마다 테스트 아닌 호출부가 있나. 파일 목록을 인자로 받는 이유는
// 테스트가 fixture 디렉터리를 가리킬 수 있게 하기 위해서다(제품 실행은 goFiles가 준다).
func check(files []string) (miss []string, notes []string, err error) {
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	for _, f := range files {
		af, e := parser.ParseFile(fset, f, nil, parser.ParseComments)
		if e != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", f, e)
		}
		parsed[f] = af
	}

	var gates []gate
	for f, af := range parsed {
		gates = append(gates, gatesIn(f, af)...)
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].Name < gates[j].Name })

	for _, g := range gates {
		if g.Pending {
			notes = append(notes, fmt.Sprintf("배선 보류: %s.%s — %s (%s)", g.Pkg, g.Name, g.Note, g.File))
			continue
		}
		if !calledOutsideTests(parsed, g) {
			miss = append(miss, fmt.Sprintf("%s: `%s.%s`를 부르는 제품 코드가 없다 — 테스트만 부른다", g.File, g.Pkg, g.Name))
		}
	}
	return miss, notes, nil
}

// gatesIn — 한 파일에서 표시된 **패키지 수준 함수**를 모은다. 리시버가 있으면 건너뛴다.
func gatesIn(file string, af *ast.File) []gate {
	var out []gate
	for _, d := range af.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Doc == nil || fn.Recv != nil {
			continue
		}
		for _, c := range fn.Doc.List {
			i := strings.Index(c.Text, marker)
			if i < 0 {
				continue
			}
			note := strings.TrimSpace(c.Text[i+len(marker):])
			out = append(out, gate{
				Name:    fn.Name.Name,
				Pkg:     af.Name.Name,
				File:    file,
				Note:    note,
				Pending: strings.HasPrefix(note, pending),
			})
			break
		}
	}
	return out
}

// calledOutsideTests — 테스트가 아닌 파일에서, 그리고 선언 파일이 아닌 곳에서 불리나.
//
// ★ 패키지로 가린다. `os.Executable()`이 이 리포에 실제로 있어서, 이름만 맞추면 **없는 배선을
// 있다고 오판한다.** 게이트가 조용히 통과하는 쪽이라 가장 위험한 오류다. 별칭 import
// (`prov "…/provisioning"`)도 놓치지 않도록 파일마다 그 패키지의 실제 이름을 먼저 푼다.
func calledOutsideTests(parsed map[string]*ast.File, g gate) bool {
	for f, af := range parsed {
		if f == g.File || strings.HasSuffix(f, "_test.go") {
			continue
		}
		samePkg := af.Name.Name == g.Pkg
		local := importNames(af, g.Pkg)
		found := false
		ast.Inspect(af, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || found {
				return !found
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				id, ok := fun.X.(*ast.Ident)
				if ok && fun.Sel.Name == g.Name && local[id.Name] {
					found = true
				}
			case *ast.Ident:
				if samePkg && fun.Name == g.Name {
					found = true
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// importNames — 이 파일이 그 패키지를 어떤 이름으로 부르나. 별칭이 있으면 별칭, 없으면 경로의
// 마지막 마디다(이 리포는 패키지 이름과 디렉터리 이름을 맞춰 쓴다).
func importNames(af *ast.File, pkg string) map[string]bool {
	out := map[string]bool{}
	for _, im := range af.Imports {
		path := strings.Trim(im.Path.Value, `"`)
		base := path[strings.LastIndex(path, "/")+1:]
		if base != pkg {
			continue
		}
		if im.Name != nil {
			out[im.Name.Name] = true
			continue
		}
		out[base] = true
	}
	return out
}

// countWired — 화면에 낼 수를 센다(등록 전체 − 보류).
func countWired(files []string) int {
	fset := token.NewFileSet()
	n := 0
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, parser.ParseComments)
		if err != nil {
			continue
		}
		for _, g := range gatesIn(f, af) {
			if !g.Pending {
				n++
			}
		}
	}
	return n
}
