// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 묶음 안에는 Go 실행 파일이 들어간다. 시험은 이 시험 실행 파일 자신을 쓴다(빌드 정보가 있다).
func selfBinary(t *testing.T) []byte {
	t.Helper()
	p, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeTarGz(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write(body)
	}
	tw.Close()
	zw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJar(t *testing.T, path, manifest string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("META-INF/MANIFEST.MF")
	w.Write([]byte(manifest))
	zw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 자산에서 만든 manifest와 같은 자산을 견주면 같고, 묶음 안의 한 바이트·파일 하나가 바뀌면 그 자리를 짚는다(M2·M3).
func TestBuildAndSame(t *testing.T) {
	dir := t.TempDir()
	bin := selfBinary(t)
	tgz := filepath.Join(dir, "pqcota-linux-amd64.tar.gz")
	jar := filepath.Join(dir, "collector.jar")
	writeTarGz(t, tgz, map[string][]byte{"pqcota-linux-amd64/pqcota-x": bin})
	writeJar(t, jar, "Implementation-Title: pqcota-discovery collector\r\nImplementation-Version: v9.9.9\r\nX-Pqcota-Commit: abc\r\nX-Pqcota-Dirty: false\r\n")

	m, err := build("v9.9.9", nil, []string{tgz, jar})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Assets) != 2 || m.Assets[0].Name != "collector.jar" || m.Assets[0].Jar.Commit != "abc" {
		t.Fatalf("assets: %+v", m.Assets)
	}
	f := m.Assets[1].Files
	if len(f) != 1 || f[0].Path != "pqcota-linux-amd64/pqcota-x" || f[0].Kind != "go" || f[0].Go == "" || f[0].Source != "buildinfo" {
		t.Fatalf("files: %+v", f)
	}
	again, _ := build("v9.9.9", nil, []string{jar, tgz})
	if err := same(m, again); err != nil {
		t.Fatalf("the same assets differ: %v", err)
	}

	// 묶음 안 실행 파일의 한 바이트
	bad := append([]byte(nil), bin...)
	bad[len(bad)/2] ^= 0xff // 끝쪽은 ELF 섹션 표라 형식이 깨진다. 가운데(코드)를 바꾼다
	writeTarGz(t, tgz, map[string][]byte{"pqcota-linux-amd64/pqcota-x": bad})
	got, err := build("v9.9.9", nil, []string{tgz, jar})
	if err != nil {
		t.Fatal(err)
	}
	if err := same(m, got); err == nil || !strings.Contains(err.Error(), "pqcota-linux-amd64/pqcota-x: differs") {
		t.Errorf("a changed byte inside the bundle was not reported: %v", err)
	}

	// manifest에 없는 파일
	writeTarGz(t, tgz, map[string][]byte{"pqcota-linux-amd64/pqcota-x": bin, "pqcota-linux-amd64/extra": bin})
	got, _ = build("v9.9.9", nil, []string{tgz, jar})
	if err := same(m, got); err == nil || !strings.Contains(err.Error(), "extra: not in the manifest") {
		t.Errorf("an extra file was not reported: %v", err)
	}
}

func releaseManifest() *Manifest {
	m := &Manifest{Format: format, Tag: "v1.0.0"}
	for _, n := range repos {
		m.Repos = append(m.Repos, Repo{Name: n, Module: modPrefix + n, Commit: "c-" + n, Sum: "h1:" + n})
	}
	m.Assets = []Asset{
		{Name: "collector.jar", Jar: &Jar{Version: "v1.0.0", Commit: "c-pqcota-discovery", Dirty: "false"}},
		{Name: "pqcota-linux-amd64.tar.gz", Files: []File{{Path: "pqcota-linux-amd64/pqcota-nodescan", Kind: "go", Modules: []Module{
			{Path: modPrefix + "pqcota-common", Version: "v1.0.0", Sum: "h1:pqcota-common"},
			{Path: modPrefix + "pqcota-discovery", Version: "v1.0.0", Sum: "h1:pqcota-discovery"},
		}}}},
	}
	return m
}

// 릴리스가 지켜야 할 것: replace가 없고, 판이 태그이고, h1:이 모듈 출처와 같고, JAR의 판·커밋이 맞다(V1·V2·V3).
func TestCheck(t *testing.T) {
	if err := check(releaseManifest(), "v1.0.0"); err != nil {
		t.Fatalf("a correct manifest failed: %v", err)
	}
	for name, mutate := range map[string]func(*Manifest){
		"replaced": func(m *Manifest) { m.Assets[1].Files[0].Modules[0].Replace = "../pqcota-common " },
		"version":  func(m *Manifest) { m.Assets[1].Files[0].Modules[0].Version = "v0.9.0" },
		"sum":      func(m *Manifest) { m.Assets[1].Files[0].Modules[1].Sum = "h1:other" },
		"jar":      func(m *Manifest) { m.Assets[0].Jar.Commit = "c-other" },
		"dirty":    func(m *Manifest) { m.Assets[0].Jar.Dirty = "true" },
		"repo":     func(m *Manifest) { m.Repos = m.Repos[1:] },
	} {
		m := releaseManifest()
		mutate(m)
		if err := check(m, "v1.0.0"); err == nil {
			t.Errorf("%s: not rejected", name)
		}
	}
	if err := check(releaseManifest(), "v1.0.1"); err == nil {
		t.Error("another tag was not rejected")
	}
}

// 커밋은 태그와 대조한 모듈 출처에서 얻는다. 프록시가 Origin을 주지 않으면 직접 받고, 그래도 확인하지 못하면 멈춘다.
func TestVerifiedOrigin(t *testing.T) {
	ok := &origin{Version: "v1.0.0", Sum: "h1:x"}
	ok.Origin = &struct{ VCS, URL, Hash, Ref string }{Hash: "c1", Ref: "refs/tags/v1.0.0"}
	none := &origin{Version: "v1.0.0", Sum: "h1:x"}

	r, err := verifiedOrigin("m", "v1.0.0", "c1", func(_, _ string, direct bool) (*origin, error) { return ok, nil })
	if err != nil || r.Source != "module-origin" || r.Commit != "c1" {
		t.Errorf("proxy origin: %+v %v", r, err)
	}
	r, err = verifiedOrigin("m", "v1.0.0", "c1", func(_, _ string, direct bool) (*origin, error) {
		if direct {
			return ok, nil
		}
		return none, nil
	})
	if err != nil || r.Source != "module-origin-direct" {
		t.Errorf("fallback to direct: %+v %v", r, err)
	}
	if _, err := verifiedOrigin("m", "v1.0.0", "c1", func(_, _ string, _ bool) (*origin, error) { return none, nil }); err == nil {
		t.Error("no Origin anywhere was accepted")
	}
	if _, err := verifiedOrigin("m", "v1.0.0", "c2", func(_, _ string, _ bool) (*origin, error) { return ok, nil }); err == nil {
		t.Error("a commit that differs from the checkout was accepted")
	}
	wrongRef := &origin{Sum: "h1:x"}
	wrongRef.Origin = &struct{ VCS, URL, Hash, Ref string }{Hash: "c1", Ref: "refs/tags/v0.9.0"}
	if _, err := verifiedOrigin("m", "v1.0.0", "c1", func(_, _ string, _ bool) (*origin, error) { return wrongRef, nil }); err == nil {
		t.Error("an origin of another tag was accepted")
	}
	if _, err := verifiedOrigin("m", "v1.0.0", "c1", func(_, _ string, _ bool) (*origin, error) { return nil, errors.New("offline") }); err == nil {
		t.Error("a download error was accepted")
	}
}
