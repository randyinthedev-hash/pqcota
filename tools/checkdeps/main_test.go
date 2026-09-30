package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const mod = "example.com/m"

// shippedRules — 실제로 함께 나가는 rules.tsv를 쓴다. 테스트에만 규칙을 적어 두면 파일이 비어도
// 케이스가 통과한다.
func shippedRules(t *testing.T) *config {
	t.Helper()
	cfg, err := loadConfig("rules.tsv", mod)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// tree — 임시 리포에 파일을 놓고 (root, 파일 목록)을 돌려준다.
func tree(t *testing.T, files map[string]string) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var list []string
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		list = append(list, name)
	}
	sort.Strings(list)
	return root, list
}

// src — 다른 패키지를 import하는 Go 파일 하나.
func src(imports ...string) string {
	var b strings.Builder
	b.WriteString("package x\n")
	for _, i := range imports {
		b.WriteString("import _ \"" + i + "\"\n")
	}
	return b.String()
}

func run(t *testing.T, files map[string]string) []string {
	t.Helper()
	root, list := tree(t, files)
	bad, _, err := check(root, list, shippedRules(t))
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// 허용된 import는 하나도 막지 않는다. 막는 것만 재면 전부 막아도 케이스가 통과한다.
func TestAllowedImportsPass(t *testing.T) {
	bad := run(t, map[string]string{
		"pkg/kernel/a/a.go":                src(mod+"/gen/pqcota/common/v1", "fmt", "google.golang.org/protobuf/proto"),
		"pkg/org/o.go":                     src(mod + "/gen/pqcota/discovery/v1"),
		"pkg/discovery/procs/p.go":         src(mod + "/gen/pqcota/discovery/v1"),
		"discovery/collectors/a/a.go":      src(mod+"/pkg/kernel/a", mod+"/pkg/discovery/procs", mod+"/discovery/collectors/b"),
		"discovery/collectors/a/a_test.go": "package a_test\nimport _ \"" + mod + "/discovery/collectors/a\"\n",
		"discovery/cmd/n/main.go":          src(mod+"/discovery/collectors/a", mod+"/pkg/inventory/render", mod+"/pkg/discovery/procs"),
		"pkg/inventory/render/r.go":        src(mod+"/pkg/kernel/a", mod+"/pkg/org", mod+"/pkg/inventory/history"),
		"pkg/inventory/history/h.go":       src(mod + "/pkg/org"),
		"pkg/provisioning/p.go":            src(mod+"/pkg/inventory/history", mod+"/pkg/kernel/a"),
		"tools/t/main.go":                  src(mod+"/pkg/inventory/render", mod+"/discovery/cmd/n"),
		"test/crossstage/doc.go":           "// Package crossstage — 통합 테스트 자리.\npackage crossstage\n",
		"test/crossstage/x_test.go":        "package crossstage_test\nimport (\n\t_ \"" + mod + "/pkg/provisioning\"\n\t_ \"" + mod + "/discovery/collectors/a\"\n\t_ \"" + mod + "/tools/t\"\n)\n",
	})
	if len(bad) != 0 {
		t.Fatalf("허용된 import가 막혔다:\n%s", strings.Join(bad, "\n"))
	}
}

// 각 규칙이 자기 위반을 잡는다. 표는 (규칙 번호, 위반 파일 → import).
func TestEachRuleCatchesItsViolation(t *testing.T) {
	cases := []struct {
		name, rule string
		files      map[string]string
	}{
		{"공통이 인벤토리를 import한다", "R1", map[string]string{"pkg/org/o.go": src(mod + "/pkg/inventory/history")}},
		{"생성물이 collector를 import한다", "R1", map[string]string{"gen/pqcota/discovery/v1/g.go": src(mod + "/discovery/collectors/a")}},
		{"테스트 파일도 같은 규칙을 받는다", "R1", map[string]string{"pkg/org/o_test.go": src(mod + "/pkg/provisioning")}},
		{"인벤토리가 디스커버리를 import한다", "R2", map[string]string{"pkg/inventory/i.go": src(mod + "/discovery/cmd/n")}},
		{"인벤토리가 procs를 import한다", "R2", map[string]string{"inventory/cmd/c/main.go": src(mod + "/pkg/discovery/procs")}},
		{"인벤토리가 프로비저닝을 import한다", "R2", map[string]string{"pkg/inventory/i.go": src(mod + "/pkg/provisioning")}},
		{"프로비저닝이 디스커버리를 import한다", "R3", map[string]string{"pkg/provisioning/p.go": src(mod + "/pkg/discovery/procs")}},
		{"디스커버리가 프로비저닝을 import한다", "R4", map[string]string{"discovery/cmd/n/main.go": src(mod + "/pkg/provisioning")}},
		{"collector가 인벤토리를 import한다", "R5", map[string]string{"discovery/collectors/a/a.go": src(mod + "/pkg/inventory/normalize")}},
		{"collector가 procs 밖의 디스커버리를 import한다", "R5", map[string]string{"discovery/collectors/a/a.go": src(mod + "/discovery/cmd/internal/localview")}},
		{"procs가 인벤토리를 import한다(간접 의존을 막는다)", "R5b", map[string]string{"pkg/discovery/procs/p.go": src(mod + "/pkg/inventory/history")}},
		{"tools가 crossstage를 import한다", "R7", map[string]string{"tools/t/main.go": src(mod + "/test/crossstage")}},
		{"인벤토리가 crossstage를 import한다", "R2", map[string]string{"pkg/inventory/i.go": src(mod + "/test/crossstage")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := run(t, c.files)
			if len(bad) != 1 || !strings.Contains(bad[0], "["+c.rule+"]") {
				t.Fatalf("규칙 %s 위반 하나가 나와야 한다: %v", c.rule, bad)
			}
		})
	}
}

// 분류표에 없는 경로는 무시하지 않고 실패시킨다. 새 디렉터리로 규칙을 피할 수 없어야 한다.
func TestUnclassifiedPathsFail(t *testing.T) {
	t.Run("import하는 쪽", func(t *testing.T) {
		bad := run(t, map[string]string{"newdir/x.go": src("fmt")})
		if len(bad) != 1 || !strings.Contains(bad[0], "분류표에 없는 경로") {
			t.Fatalf("미분류 디렉터리가 통과했다: %v", bad)
		}
	})
	t.Run("import되는 쪽", func(t *testing.T) {
		bad := run(t, map[string]string{"pkg/kernel/a/a.go": src(mod + "/mystery")})
		if len(bad) != 1 || !strings.Contains(bad[0], "mystery") {
			t.Fatalf("미분류 import가 통과했다: %v", bad)
		}
	})
	t.Run("모듈 루트", func(t *testing.T) {
		bad := run(t, map[string]string{"main.go": src("fmt")})
		if len(bad) != 1 {
			t.Fatalf("루트 파일이 통과했다: %v", bad)
		}
	})
}

// crossstage는 doc.go와 _test.go만 둔다. doc.go에 구현이 들어오는 것은 import만 읽어서는 못 막는다.
func TestCrossstageIsBare(t *testing.T) {
	good := "// Package crossstage — 자리.\npackage crossstage\n"
	cases := []struct {
		name, want string
		files      map[string]string
	}{
		{"구현 파일", "doc.go와 _test.go만", map[string]string{"test/crossstage/doc.go": good, "test/crossstage/impl.go": src()}},
		{"doc.go에 함수", "패키지 문서와 package 선언만", map[string]string{"test/crossstage/doc.go": good + "func F() {}\n"}},
		{"doc.go에 import", "패키지 문서와 package 선언만", map[string]string{"test/crossstage/doc.go": "// Package crossstage — 자리.\npackage crossstage\nimport _ \"fmt\"\n"}},
		{"doc.go에 문서 주석 없음", "패키지 문서 주석이 없다", map[string]string{"test/crossstage/doc.go": "package crossstage\n"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := run(t, c.files)
			if len(bad) == 0 || !strings.Contains(strings.Join(bad, "\n"), c.want) {
				t.Fatalf("%q가 나와야 한다: %v", c.want, bad)
			}
		})
	}
	if bad := run(t, map[string]string{"test/crossstage/doc.go": good}); len(bad) != 0 {
		t.Fatalf("정상 doc.go가 막혔다: %v", bad)
	}
}

// testdata는 Go 도구도 보지 않는 자리다. 이 검사기 자신의 fixture가 관문을 깨면 안 된다.
func TestTestdataIsSkipped(t *testing.T) {
	bad := run(t, map[string]string{"pkg/org/testdata/bad.go": src(mod + "/pkg/inventory/history")})
	if len(bad) != 0 {
		t.Fatalf("testdata가 재어졌다: %v", bad)
	}
}

// 분류: 가장 긴 접두어가 이긴다. 이름이 discovery여도 gen은 공통 계약이다.
func TestClassification(t *testing.T) {
	cfg := shippedRules(t)
	for dir, want := range map[string]string{
		"gen/pqcota/discovery/v1":           "common",
		"gen/pqcota/inventory/v1":           "common",
		"pkg/kernel/completeness":           "common",
		"pkg/org":                           "common",
		"pkg/discovery/procs":               "procs",
		"pkg/discovery/other":               "discovery",
		"discovery/collectors/openssl":      "collector",
		"discovery/cmd/pqcota-nodescan":     "discovery",
		"discovery/cmd/internal/localview":  "discovery",
		"pkg/inventory/history":             "inventory",
		"inventory/cmd/pqcota-ingest":       "inventory",
		"pkg/provisioning":                  "provisioning",
		"provisioning/cmd/pqcota-provision": "provisioning",
		"test/crossstage":                   "crossstage",
		"tools/checkdeps":                   "leaf",
		"demo/topology/topogen":             "leaf",
		"":                                  "",
		"unknown/dir":                       "",
	} {
		if got := cfg.areaOf(dir); got != want {
			t.Errorf("%q → %q, 기대 %q", dir, got, want)
		}
	}
}

// rules.tsv를 잘못 적으면 알리지 않고 넘기지 않고 멈춘다. 넘기면 그 규칙이 사라져 관문이 헐거워진다.
func TestRulesFileErrors(t *testing.T) {
	head := "class\tpkg/a/\tx\nallow\tx\tx\tR9\n"
	for name, body := range map[string]string{
		"접두어에 / 없음":         "class\tpkg/a\tx\nallow\tx\tx\tR9\n",
		"모르는 줄 종류":          head + "wat\tx\n",
		"allow가 모르는 영역을 허용": "class\tpkg/a/\tx\nallow\tx\tx,y\tR9\n",
		"영역에 allow 줄 없음":    "class\tpkg/a/\tx\nclass\tpkg/b/\ty\nallow\tx\tx\tR9\n",
		"class 열 수 틀림":      "class\tpkg/a/\n",
		"bare가 모르는 영역":      head + "bare\tz\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "rules.tsv")
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := loadConfig(p, mod); err == nil {
				t.Fatal("잘못된 rules.tsv가 통과했다")
			}
		})
	}
	if _, err := loadConfig("rules.tsv", mod); err != nil {
		t.Fatalf("함께 나가는 rules.tsv가 읽히지 않는다: %v", err)
	}
}

// 이 리포 자신이 규칙을 지킨다. 위 케이스들은 만든 입력으로 재는 것이라, 진짜 리포에서 맞는지는 여기서 잰다.
// 분리된 형제 리포가 옆에 있으면 함께 잰다(작업 공간 배치). 없으면 이 리포만 잰다.
func TestThisRepoPasses(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	roots := []string{root}
	for _, sib := range []string{"pqcota-common", "pqcota-inventory", "pqcota-discovery", "pqcota-provisioning"} {
		p := filepath.Join(filepath.Dir(root), sib)
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
			roots = append(roots, p)
		}
	}
	var modules []string
	for _, r := range roots {
		m, err := moduleOf(filepath.Join(r, "go.mod"))
		if err != nil {
			t.Fatal(err)
		}
		modules = append(modules, m)
	}
	cfg, err := loadConfig("rules.tsv", modules...)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, r := range roots {
		cmd := exec.Command("git", "-c", "core.quotePath=off", "ls-files", "*.go")
		cmd.Dir = r
		out, err := cmd.Output()
		if err != nil {
			t.Skipf("git ls-files를 못 돌린다(체크아웃이 아니다): %v", err)
		}
		bad, n, err := check(r, strings.Fields(string(out)), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(bad) != 0 {
			t.Fatalf("%s가 규칙을 어긴다:\n%s", filepath.Base(r), strings.Join(bad, "\n"))
		}
		total += n
	}
	if total == 0 {
		t.Fatal("모듈 내부 import가 하나도 안 잡혔다. 관문이 아무것도 재지 않는다")
	}
}
