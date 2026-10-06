// SPDX-FileCopyrightText: 2026 randyinthedev
// SPDX-License-Identifier: Apache-2.0

// Command checkdeps — **단계 사이의 import 방향**을 막는다.
//
// 이 리포는 인벤토리를 허브로 둔다. 디스커버리와 프로비저닝이 인벤토리를 참조하고, 인벤토리는 두
// 단계 어느 쪽도 참조하지 않으며, 공통(contracts·kernel·org)은 어느 단계도 참조하지 않는다.
// 「kernel은 단계를 import하지 않는다」는 규칙이 문서에만 있었고 그것을 지키는 기계가
// 없었으며, 실제로 인벤토리가 디스커버리 디렉터리에서 자기 저장소를 가져오는 방향이 한동안 반대였다.
//
// **리포가 여럿이어도 규칙표는 하나다.** 분리된 리포마다 원래 상대 경로(gen/·pkg/kernel/·discovery/ 등)를
// 그대로 지키므로, 파일의 영역은 리포 안 경로로 정하고, 형제 모듈을 가리키는 import는 그 모듈 경로를
// 떼어 낸 나머지 경로로 정한다. 돌릴 때 루트를 여러 개 준다(`checkdeps [-rules f] <root>...`).
//
// 규칙은 코드가 아니라 rules.tsv에 있다(경로 분류표와, 영역마다 import해도 되는 영역의 허용 목록).
// **금지 목록이 아니라 허용 목록**이다. 금지 목록은 새 영역이나 새 경로가 생기는 날 조용히 통과한다.
// 그래서 분류표에 없는 모듈 내부 경로도 실패로 센다.
//
// 재는 것은 **패키지 사이의 직접 import**다. 테스트 파일도 같은 영역의 규칙을 받고, 외부 테스트
// 패키지(foo_test)는 같은 디렉터리라 같은 영역이다. 표준 라이브러리와 외부 모듈은 재지 않는다
// (라이선스는 checklicenses가 잰다).
//
// **재지 못하는 것**: 배포 명령이 링크하는 의존 폐포(예: pqcota-nodescan이 localview를 거쳐 인벤토리와
// pgx를 링크한다)와 규칙에 없는 전이 경로. procs처럼 영역을 좁게 잠근 곳만 직접 규칙이 곧 전이
// 규칙이다. 못 보는 것을 안 보는 척하지 않는다.
//
// bare로 지정한 영역(test/crossstage)에는 doc.go와 _test.go만 둘 수 있고, doc.go는 패키지 문서와
// package 선언만 가져야 한다. import만 읽는 검사로는 그 파일에 구현이 들어오는 것을 못 막으므로
// 그 파일은 전체를 파싱한다.
package main

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type class struct{ repo, prefix, area string } // repo가 비면 모든 리포에 적용된다

type allowRow struct {
	rule  string
	hint  string
	areas map[string]bool
}

type config struct {
	modules []string // 함께 재는 모듈 경로들. 이 가운데 하나를 접두어로 하는 import가 「모듈 내부」다
	self    string   // 지금 재는 리포 이름(모듈 경로의 마지막 마디). 리포로 한정한 class가 이것과 맞는다
	classes []class  // 접두어가 긴 순
	allow   map[string]allowRow
	bare    map[string]bool
	areas   map[string]bool
}

// loadConfig — rules.tsv를 읽는다. 잘못된 줄은 알리지 않고 넘기지 않고 오류로 멈춘다:
// 넘기면 그 규칙이 사라져 관문이 조용히 헐거워진다.
func loadConfig(rulesPath string, modules ...string) (*config, error) {
	f, err := os.Open(rulesPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg := &config{modules: modules, allow: map[string]allowRow{}, bare: map[string]bool{}, areas: map[string]bool{}}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		col := strings.Split(line, "\t")
		switch col[0] {
		case "class":
			if len(col) != 3 {
				return nil, fmt.Errorf("%s:%d: class는 「class<탭>[리포:]접두어/<탭>영역」 꼴이다", rulesPath, n)
			}
			repo, prefix := "", col[1]
			if i := strings.Index(prefix, ":"); i >= 0 {
				repo, prefix = prefix[:i], prefix[i+1:]
				if repo == "" {
					return nil, fmt.Errorf("%s:%d: class의 리포 한정자가 비었다", rulesPath, n)
				}
			}
			if !strings.HasSuffix(prefix, "/") {
				return nil, fmt.Errorf("%s:%d: class는 「class<탭>[리포:]접두어/<탭>영역」 꼴이다", rulesPath, n)
			}
			cfg.classes = append(cfg.classes, class{repo, prefix, col[2]})
			cfg.areas[col[2]] = true
		case "allow":
			if len(col) < 4 || len(col) > 5 {
				return nil, fmt.Errorf("%s:%d: allow는 「allow<탭>영역<탭>허용 영역<탭>규칙[<탭>고치는 방향]」 꼴이다", rulesPath, n)
			}
			row := allowRow{rule: col[3], areas: map[string]bool{}}
			if len(col) == 5 {
				row.hint = col[4]
			}
			for _, a := range strings.Split(col[2], ",") {
				row.areas[strings.TrimSpace(a)] = true
			}
			cfg.allow[col[1]] = row
		case "bare":
			if len(col) != 2 {
				return nil, fmt.Errorf("%s:%d: bare는 「bare<탭>영역」 꼴이다", rulesPath, n)
			}
			cfg.bare[col[1]] = true
		default:
			return nil, fmt.Errorf("%s:%d: 모르는 줄 종류 %q", rulesPath, n, col[0])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// 분류에 없는 영역을 규칙이 가리키면 그 규칙은 영영 안 맞는다: 오타를 막는다.
	for a, row := range cfg.allow {
		if !cfg.areas[a] {
			return nil, fmt.Errorf("%s: allow가 분류에 없는 영역 %q를 다룬다", rulesPath, a)
		}
		for t := range row.areas {
			if !cfg.areas[t] {
				return nil, fmt.Errorf("%s: allow %s가 분류에 없는 영역 %q를 허용한다", rulesPath, a, t)
			}
		}
	}
	for a := range cfg.areas {
		if _, ok := cfg.allow[a]; !ok {
			return nil, fmt.Errorf("%s: 영역 %q에 allow 줄이 없다. 없으면 그 영역은 아무것도 import할 수 없게 된다", rulesPath, a)
		}
	}
	for a := range cfg.bare {
		if !cfg.areas[a] {
			return nil, fmt.Errorf("%s: bare가 분류에 없는 영역 %q를 다룬다", rulesPath, a)
		}
	}
	// 가장 긴 접두어가 이기고, 같은 길이면 리포로 한정한 것이 먼저다.
	sort.SliceStable(cfg.classes, func(i, j int) bool {
		a, b := cfg.classes[i], cfg.classes[j]
		if len(a.prefix) != len(b.prefix) {
			return len(a.prefix) > len(b.prefix)
		}
		return a.repo != "" && b.repo == ""
	})
	return cfg, nil
}

// relOf — import 경로가 이 규칙이 다루는 모듈 안이면 그 모듈 루트를 뗀 상대 경로와 리포 이름을 돌려준다.
// 표준 라이브러리와 외부 모듈이면 false다. 모듈 경로가 서로 접두어일 수 있어(`…/pqcota`와
// `…/pqcota-common`) 경로 마디 단위로 맞춘다.
func (c *config) relOf(p string) (rel, repo string, ok bool) {
	for _, m := range c.modules {
		if p == m {
			return "", path.Base(m), true
		}
		if strings.HasPrefix(p, m+"/") {
			return strings.TrimPrefix(p, m+"/"), path.Base(m), true
		}
	}
	return "", "", false
}

// areaIn — 리포 repo 안의 패키지 디렉터리(리포 상대, 슬래시)가 어느 영역인가. 어디에도 안 걸리면 빈 문자열이다.
// 리포로 한정한 class(`pqcota-discovery:cmd/`)는 그 리포에서만 맞는다 — 모든 단계 리포가 `cmd/`를 가지므로
// 경로만으로는 어느 단계의 것인지 가릴 수 없다.
func (c *config) areaIn(repo, dir string) string {
	d := strings.Trim(dir, "/") + "/"
	if d == "/" {
		return "" // 모듈 루트
	}
	for _, cl := range c.classes {
		if strings.HasPrefix(d, cl.prefix) && (cl.repo == "" || cl.repo == repo) {
			return cl.area
		}
	}
	return ""
}

// areaOf — 지금 재는 리포 안의 디렉터리가 어느 영역인가.
func (c *config) areaOf(dir string) string { return c.areaIn(c.self, dir) }

// check — 파일 목록(리포 상대 경로)을 재어 위반을 돌려준다. root는 파일을 읽을 자리다.
// 파일 목록을 인자로 받는 것은 테스트가 fixture 디렉터리를 가리킬 수 있게 하려는 것이다.
func check(root string, files []string, cfg *config) ([]string, int, error) {
	var out []string
	imports := 0
	unclassifiedDirs := map[string]bool{}
	fset := token.NewFileSet()
	for _, f := range files {
		if hasDir(f, "testdata") {
			continue // Go 도구도 보지 않는 자리다. 이 검사기 자신의 fixture가 여기 산다
		}
		dir := path.Dir(f)
		if dir == "." {
			dir = ""
		}
		area := cfg.areaOf(dir)
		if area == "" {
			if !unclassifiedDirs[dir] {
				unclassifiedDirs[dir] = true
				out = append(out, fmt.Sprintf("%s: 분류표에 없는 경로다. rules.tsv에 class 줄을 더해 영역을 정해야 한다", f))
			}
			continue
		}
		base := path.Base(f)
		if cfg.bare[area] && base != "doc.go" && !strings.HasSuffix(base, "_test.go") {
			out = append(out, fmt.Sprintf("%s: %s 영역에는 doc.go와 _test.go만 둔다. 구현 코드는 다른 곳에 둘 것", f, area))
			continue
		}
		mode := parser.ImportsOnly | parser.ParseComments
		if cfg.bare[area] && base == "doc.go" {
			mode = parser.ParseComments // 선언이 들어왔는지 봐야 하므로 전체를 읽는다
		}
		file, err := parser.ParseFile(fset, root+"/"+f, nil, mode)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", f, err)
		}
		if cfg.bare[area] && base == "doc.go" {
			if file.Doc == nil {
				out = append(out, fmt.Sprintf("%s: doc.go에 패키지 문서 주석이 없다", f))
			}
			if len(file.Decls) > 0 {
				pos := fset.Position(file.Decls[0].Pos())
				out = append(out, fmt.Sprintf("%s:%d: doc.go에는 패키지 문서와 package 선언만 둔다. import·선언이 들어 있다", f, pos.Line))
			}
		}
		row := cfg.allow[area]
		for _, imp := range file.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, 0, fmt.Errorf("%s: %w", f, err)
			}
			rel, irepo, internal := cfg.relOf(p)
			if !internal {
				continue // 표준 라이브러리와 외부 모듈은 재지 않는다
			}
			imports++
			to := cfg.areaIn(irepo, rel)
			if irepo != cfg.self {
				rel = irepo + "/" + rel // 다른 리포의 것은 어느 리포인지 함께 보인다
			}
			line := fset.Position(imp.Pos()).Line
			switch {
			case to == "":
				out = append(out, fmt.Sprintf("%s:%d: 분류표에 없는 경로를 import한다: %s (rules.tsv에 class 줄을 더해야 한다)", f, line, rel))
			case !row.areas[to]:
				msg := fmt.Sprintf("%s:%d: [%s] %s 영역에서 %s(%s 영역)를 import할 수 없다", f, line, row.rule, area, rel, to)
				if row.hint != "" {
					msg += ". " + row.hint
				}
				out = append(out, msg)
			}
		}
	}
	sort.Strings(out)
	return out, imports, nil
}

func hasDir(p, name string) bool {
	for _, seg := range strings.Split(path.Dir(p), "/") {
		if seg == name {
			return true
		}
	}
	return false
}

func moduleOf(gomod string) (string, error) {
	b, err := os.ReadFile(gomod)
	if err != nil {
		return "", err
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(ln, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(ln, "module ")), nil
		}
	}
	return "", fmt.Errorf("%s에 module 줄이 없다", gomod)
}

func goFiles(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "-c", "core.quotePath=off", "ls-files", "*.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files (%s): %w", root, err)
	}
	return strings.Fields(string(out)), nil
}

func main() {
	rules := "tools/checkdeps/rules.tsv"
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "-rules" {
		rules, args = args[1], args[2:]
	}
	roots := args
	if len(roots) == 0 {
		roots = []string{"."}
	}
	var modules []string
	for _, r := range roots {
		m, err := moduleOf(filepath.Join(r, "go.mod"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "go.mod:", err)
			os.Exit(2)
		}
		modules = append(modules, m)
	}
	cfg, err := loadConfig(rules, modules...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var bad []string
	nFiles, nImports := 0, 0
	for i, r := range roots {
		cfg.self = path.Base(modules[i])
		files, err := goFiles(r)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		b, n, err := check(r, files, cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		nFiles += len(files)
		nImports += n
		for _, line := range b {
			if len(roots) > 1 {
				line = filepath.Base(filepath.Clean(r)) + "/" + line
			}
			bad = append(bad, line)
		}
	}
	if len(bad) > 0 {
		fmt.Println("✗ 단계 사이의 import 방향이 어긋난다:")
		for _, b := range bad {
			fmt.Println("    " + b)
		}
		fmt.Println()
		fmt.Println("deps check failed — fix the locations above and run `make check-deps` again.")
		os.Exit(1)
	}
	fmt.Printf("✓ deps check passed (리포 %d개 · Go 파일 %d개 · 모듈 내부 import %d건 · 영역 %d개)\n", len(roots), nFiles, nImports, len(cfg.areas))
}
