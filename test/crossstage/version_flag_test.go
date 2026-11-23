// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// commands — 네 단계 리포의 명령 전부(main 패키지). 손으로 적지 않는다: --version이 없는 새 명령이 생기면
// 이 목록에 들어와 아래 시험에 걸린다.
func commands(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{if eq .Name "main"}}{{.ImportPath}}{{end}}`,
		"github.com/randyinthedev-hash/pqcota-common/cmd/...",
		"github.com/randyinthedev-hash/pqcota-discovery/cmd/...",
		"github.com/randyinthedev-hash/pqcota-inventory/cmd/...",
		"github.com/randyinthedev-hash/pqcota-provisioning/cmd/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	var pkgs []string
	for _, l := range strings.Fields(string(out)) {
		pkgs = append(pkgs, l)
	}
	if len(pkgs) < 19 {
		t.Fatalf("found %d commands, expected at least 19: %v", len(pkgs), pkgs)
	}
	return pkgs
}

// buildAll — 명령을 한 폴더에 한 번에 빌드한다.
func buildAll(t *testing.T, pkgs []string) string {
	t.Helper()
	dir := t.TempDir()
	args := append([]string{"build", "-o", dir + string(os.PathSeparator)}, pkgs...)
	if out, err := exec.Command("go", args...).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return dir
}

type run struct {
	code           int
	stdout, stderr string
}

func runIn(t *testing.T, bin, cwd string, env []string, args ...string) run {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	cmd.Env = env
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v", bin, err)
	}
	return run{code, o.String(), e.String()}
}

// brokenEnv — 잘못된 키 설정과 연결할 수 없는 DSN. 단독 --version은 이것들을 읽기 전에 끝나야 한다.
func brokenEnv() []string {
	return append(os.Environ(),
		"PQCOTA_SIGN_KEY=not-a-key",
		"PQCOTA_VERIFY_KEY=not-a-key",
		"PQCOTA_VERIFY_KEY_FILE=/nonexistent/pqcota-verify-keys",
		"PQCOTA_APPROVAL_KEYS=not-a-key",
		"PQCOTA_DSN=postgres://pqcota:pqcota@127.0.0.1:1/pqcota?sslmode=disable&connect_timeout=1",
	)
}

// V3: 모든 명령이 단독 --version·-version에 종료 0, 한 줄(명령 이름으로 시작), 표준 오류 없음. 잘못된 키와
// 연결할 수 없는 DSN을 둔 상태에서도 그렇고, 빈 작업 폴더에 아무것도 생기지 않는다. 이 시험은 replace로
// 옆 폴더를 읽는 로컬 빌드이므로, 릴리스가 아니라고 적혀야 한다.
func TestVersionFlagAllCommands(t *testing.T) {
	pkgs := commands(t)
	dir := buildAll(t, pkgs)
	env := brokenEnv()
	for _, pkg := range pkgs {
		name := filepath.Base(pkg)
		for _, flag := range []string{"--version", "-version"} {
			cwd := t.TempDir()
			r := runIn(t, filepath.Join(dir, name), cwd, env, flag)
			lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
			if r.code != 0 || r.stderr != "" || len(lines) != 1 || !strings.HasPrefix(lines[0], name+" ") {
				t.Errorf("%s %s: exit %d, stdout %q, stderr %q", name, flag, r.code, r.stdout, r.stderr)
				continue
			}
			if !strings.Contains(lines[0], "not a release build") {
				t.Errorf("%s %s: a local build reads like a release: %q", name, flag, lines[0])
			}
			if ents, _ := os.ReadDir(cwd); len(ents) != 0 {
				t.Errorf("%s %s: wrote into the working directory: %v", name, flag, ents)
			}
		}
	}
}

// V3: 섞인 호출은 기존 인자 처리 규칙을 따른다 — --version이 파일 이름이나 값일 수 있다.
func TestVersionFlagMixedCallsKeepTheirRules(t *testing.T) {
	dir := buildAll(t, []string{
		"github.com/randyinthedev-hash/pqcota-discovery/cmd/pqcota-nodescan",
		"github.com/randyinthedev-hash/pqcota-inventory/cmd/pqcota-cbom-ingest",
		"github.com/randyinthedev-hash/pqcota-inventory/cmd/pqcota-discover-view",
	})
	env := brokenEnv()
	notVersion := func(name string, r run) {
		t.Helper()
		if strings.HasPrefix(r.stdout, name+" ") && strings.Contains(r.stdout, "(go") {
			t.Errorf("%s: a mixed call printed the version line: %q", name, r.stdout)
		}
	}

	// flag를 쓰는 명령: version 플래그를 등록하지 않았으므로 지금처럼 정의되지 않은 플래그다.
	r := runIn(t, filepath.Join(dir, "pqcota-nodescan"), t.TempDir(), env, "--version", "web-01")
	notVersion("pqcota-nodescan", r)
	if r.code != 2 || !strings.Contains(r.stderr, "flag provided but not defined: -version") {
		t.Errorf("pqcota-nodescan --version web-01: exit %d, stderr %q", r.code, r.stderr)
	}

	// 위치 인자 명령: --version은 그런 이름의 파일이다. 있으면 그 파일을 읽는다.
	cwd := t.TempDir()
	r = runIn(t, filepath.Join(dir, "pqcota-cbom-ingest"), cwd, env, "--version", "web-01")
	notVersion("pqcota-cbom-ingest", r)
	if r.code == 0 || !strings.Contains(r.stderr, "--version") {
		t.Errorf("pqcota-cbom-ingest --version web-01 (no such file): exit %d, stderr %q", r.code, r.stderr)
	}
	if err := os.WriteFile(filepath.Join(cwd, "--version"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = runIn(t, filepath.Join(dir, "pqcota-cbom-ingest"), cwd, env, "--version", "web-01")
	notVersion("pqcota-cbom-ingest", r)
	if r.code == 0 || strings.Contains(r.stderr, "no such file") {
		t.Errorf("pqcota-cbom-ingest --version web-01 (file exists): exit %d, stderr %q — the file was not read", r.code, r.stderr)
	}

	// 위치 인자 명령: --version은 결과 폴더 이름이다.
	cwd = t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, "--version"), 0o755); err != nil {
		t.Fatal(err)
	}
	r = runIn(t, filepath.Join(dir, "pqcota-discover-view"), cwd, env, "--version", "nodes-missing.json")
	notVersion("pqcota-discover-view", r)
}
