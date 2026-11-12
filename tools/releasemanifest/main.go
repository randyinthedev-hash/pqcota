// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Command releasemanifest — 릴리스 manifest(`pqcota-release-manifest.json`)를 만들고 대조한다.
//
//	releasemanifest generate -tag <태그> -checkouts <다섯 리포가 나란히 있는 폴더> -o <manifest> <자산…>
//	releasemanifest verify   -manifest <manifest> [-tag <태그>] <자산…>
//
// 자산은 압축 묶음(.tar.gz·.zip)과 collector.jar다. manifest는 자기 자신과 SHA256SUMS의 해시를 담지 않는다
// (SHA256SUMS가 manifest를 덮는다 — 순서는 manifest → SHA256SUMS).
//
// **값의 출처를 가른다**(계획 release-provenance §4.1.1).
//   - Go 실행 파일의 모듈 판·`h1:`은 그 파일의 빌드 정보에서 읽는다(`source: buildinfo`).
//   - JAR의 판·커밋은 JAR의 manifest에서 읽는다(`source: jar-manifest`).
//   - 리포마다의 태그 커밋은 태그와 대조한 모듈 출처에서 얻는다(`go mod download -json`의 `Origin`,
//     `source: module-origin`). 프록시가 `Origin`을 주지 않으면 저장소에서 직접 받는다(GOPROXY=direct).
//     그래도 커밋을 확인하지 못하거나 체크아웃 커밋과 다르면 **만들지 않고 멈춘다.**
//
// generate는 만든 뒤 같은 규칙으로 verify까지 해서, 어긋난 manifest를 내보내지 않는다.
package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
)

const (
	format    = "pqcota-release-manifest/1"
	modPrefix = "github.com/randyinthedev-hash/"
)

// repos — 한 릴리스를 이루는 다섯 리포. 모듈 경로는 modPrefix + 이름이다.
var repos = []string{"pqcota", "pqcota-common", "pqcota-inventory", "pqcota-discovery", "pqcota-provisioning"}

type Manifest struct {
	Format string  `json:"format"`
	Tag    string  `json:"tag"`
	Go     string  `json:"go"`
	Repos  []Repo  `json:"repos"`
	Assets []Asset `json:"assets"`
}

// Repo — 리포의 태그 커밋. 출처는 모듈 출처(Origin)이고, 워크플로의 체크아웃 커밋과 같음을 확인했다.
type Repo struct {
	Name      string `json:"name"`
	Module    string `json:"module"`
	Commit    string `json:"commit"`
	OriginRef string `json:"origin_ref"`
	Sum       string `json:"sum"`
	Source    string `json:"source"` // "module-origin" 또는 "module-origin-direct"
}

type Asset struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Files  []File `json:"files,omitempty"` // 압축 묶음 안의 파일. collector.jar 자산은 비고 Jar가 찬다
	Jar    *Jar   `json:"jar,omitempty"`
}

type File struct {
	Path    string   `json:"path"`
	SHA256  string   `json:"sha256"`
	Kind    string   `json:"kind"` // "go"
	Go      string   `json:"go"`
	Main    string   `json:"main"` // main 패키지 경로
	Modules []Module `json:"modules"`
	Source  string   `json:"source"` // "buildinfo"
}

// Module — 빌드 정보의 pqcota 모듈 한 줄(main 모듈 또는 의존).
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum"`
	Replace string `json:"replace,omitempty"` // replace가 있으면 그 대상. 릴리스에서는 늘 비어야 한다
}

type Jar struct {
	Title   string `json:"title"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Dirty   string `json:"dirty"`
	Source  string `json:"source"` // "jar-manifest"
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "generate":
		err = cmdGenerate(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ releasemanifest:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: releasemanifest generate -tag T -checkouts DIR -o MANIFEST ASSET...\n       releasemanifest verify -manifest MANIFEST [-tag T] ASSET...")
	os.Exit(2)
}

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	tag := fs.String("tag", "", "release tag")
	checkouts := fs.String("checkouts", "..", "folder holding the five repository checkouts side by side")
	out := fs.String("o", "pqcota-release-manifest.json", "manifest to write")
	fs.Parse(args)
	if *tag == "" || fs.NArg() == 0 {
		usage()
	}
	rs, err := repoCommits(*tag, *checkouts, goModDownload)
	if err != nil {
		return err
	}
	m, err := build(*tag, rs, fs.Args())
	if err != nil {
		return err
	}
	if err := check(m, *tag); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		return err
	}
	n := 0
	for _, a := range m.Assets {
		n += len(a.Files)
	}
	fmt.Printf("✓ %s: %d assets, %d files inside, %d repositories (tag %s)\n", *out, len(m.Assets), n, len(m.Repos), *tag)
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	path := fs.String("manifest", "pqcota-release-manifest.json", "manifest to check against")
	tag := fs.String("tag", "", "require this tag (default: the manifest's)")
	fs.Parse(args)
	if fs.NArg() == 0 {
		usage()
	}
	b, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	var want Manifest
	if err := json.Unmarshal(b, &want); err != nil {
		return fmt.Errorf("%s: %w", *path, err)
	}
	if *tag == "" {
		*tag = want.Tag
	}
	got, err := build(want.Tag, want.Repos, fs.Args())
	if err != nil {
		return err
	}
	if err := check(got, *tag); err != nil {
		return err
	}
	if err := same(&want, got); err != nil {
		return err
	}
	fmt.Printf("✓ %d assets match %s (tag %s)\n", len(got.Assets), filepath.Base(*path), *tag)
	return nil
}

// build — 자산을 읽어 manifest를 만든다. repos는 그대로 싣는다(generate는 모듈 출처에서, verify는 대조할
// manifest에서 받는다). 자산 순서는 이름순이다.
func build(tag string, rs []Repo, assets []string) (*Manifest, error) {
	m := &Manifest{Format: format, Tag: tag, Repos: rs}
	sort.Strings(assets)
	goVer := ""
	for _, p := range assets {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		a := Asset{Name: filepath.Base(p), SHA256: sum(b)}
		switch {
		case strings.HasSuffix(p, ".tar.gz"):
			a.Files, err = tarFiles(b)
		case strings.HasSuffix(p, ".zip"):
			a.Files, err = zipFiles(b)
		case strings.HasSuffix(p, ".jar"):
			a.Jar, err = jarInfo(b)
		default:
			err = fmt.Errorf("unknown asset type")
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.Name, err)
		}
		for _, f := range a.Files {
			if goVer == "" {
				goVer = f.Go
			} else if f.Go != goVer {
				return nil, fmt.Errorf("%s/%s: built with %s, others with %s", a.Name, f.Path, f.Go, goVer)
			}
		}
		m.Assets = append(m.Assets, a)
	}
	m.Go = goVer
	return m, nil
}

func tarFiles(b []byte) ([]File, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	var out []File
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		f, err := goFile(h.Name, body)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func zipFiles(b []byte) ([]File, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	var out []File
	for _, e := range zr.File {
		if e.FileInfo().IsDir() {
			continue
		}
		rc, err := e.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		f, err := goFile(e.Name, body)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// goFile — 묶음 안의 파일 하나. 묶음에는 Go 실행 파일만 들어간다 — 빌드 정보가 없으면 오류다.
func goFile(name string, body []byte) (File, error) {
	bi, err := buildinfo.Read(bytes.NewReader(body))
	if err != nil {
		return File{}, fmt.Errorf("%s: no Go build information: %w", name, err)
	}
	f := File{Path: strings.TrimPrefix(name, "./"), SHA256: sum(body), Kind: "go", Go: bi.GoVersion, Main: bi.Path, Source: "buildinfo"}
	add := func(m *debug.Module) {
		if m == nil || !strings.HasPrefix(m.Path, modPrefix) {
			return
		}
		x := Module{Path: m.Path, Version: m.Version, Sum: m.Sum}
		if m.Replace != nil {
			x.Replace = m.Replace.Path + " " + m.Replace.Version
		}
		f.Modules = append(f.Modules, x)
	}
	add(&bi.Main)
	for _, d := range bi.Deps {
		add(d)
	}
	sort.Slice(f.Modules, func(i, j int) bool { return f.Modules[i].Path < f.Modules[j].Path })
	return f, nil
}

func jarInfo(b []byte) (*Jar, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	for _, e := range zr.File {
		if e.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		rc, err := e.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		attrs := map[string]string{}
		sc := bufio.NewScanner(rc)
		for sc.Scan() {
			k, v, ok := strings.Cut(strings.TrimRight(sc.Text(), "\r"), ": ")
			if ok {
				attrs[k] = v
			}
		}
		return &Jar{Title: attrs["Implementation-Title"], Version: attrs["Implementation-Version"],
			Commit: attrs["X-Pqcota-Commit"], Dirty: attrs["X-Pqcota-Dirty"], Source: "jar-manifest"}, nil
	}
	return nil, fmt.Errorf("no META-INF/MANIFEST.MF")
}

// check — 릴리스 manifest가 지켜야 할 것(V1·V3, 그리고 출처끼리의 대조).
func check(m *Manifest, tag string) error {
	if m.Tag != tag {
		return fmt.Errorf("manifest tag %s, want %s", m.Tag, tag)
	}
	byMod := map[string]Repo{}
	for _, r := range m.Repos {
		byMod[r.Module] = r
	}
	for _, name := range repos {
		r, ok := byMod[modPrefix+name]
		if !ok || r.Commit == "" {
			return fmt.Errorf("%s: no verified tag commit", name)
		}
	}
	var errs []string
	for _, a := range m.Assets {
		if a.Jar != nil {
			j := a.Jar
			disc := byMod[modPrefix+"pqcota-discovery"]
			if j.Version != tag || j.Commit != disc.Commit || j.Dirty != "false" {
				errs = append(errs, fmt.Sprintf("%s: version=%q commit=%q dirty=%q, want %s, %s, false", a.Name, j.Version, j.Commit, j.Dirty, tag, disc.Commit))
			}
			continue
		}
		if len(a.Files) == 0 {
			errs = append(errs, a.Name+": no files")
		}
		for _, f := range a.Files {
			if len(f.Modules) == 0 {
				errs = append(errs, fmt.Sprintf("%s/%s: no pqcota module in the build information", a.Name, f.Path))
			}
			for _, mod := range f.Modules {
				r := byMod[mod.Path]
				switch {
				case mod.Replace != "":
					errs = append(errs, fmt.Sprintf("%s/%s: %s is replaced by %s", a.Name, f.Path, mod.Path, mod.Replace))
				case mod.Version != tag:
					errs = append(errs, fmt.Sprintf("%s/%s: %s %s, want %s", a.Name, f.Path, mod.Path, mod.Version, tag))
				case mod.Sum == "" || mod.Sum != r.Sum:
					errs = append(errs, fmt.Sprintf("%s/%s: %s sum %q, module origin %q", a.Name, f.Path, mod.Path, mod.Sum, r.Sum))
				}
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d problem(s):\n  %s", len(errs), strings.Join(errs, "\n  "))
	}
	return nil
}

// same — 대조할 manifest와 자산에서 다시 만든 것이 같은가. 자산 이름·해시, 묶음 안 파일 목록·해시·빌드 정보,
// JAR 값을 모두 본다. 다르면 어디가 다른지 적는다.
func same(want, got *Manifest) error {
	w, _ := json.Marshal(want.Assets)
	g, _ := json.Marshal(got.Assets)
	if bytes.Equal(w, g) && want.Go == got.Go {
		return nil
	}
	wa := map[string]Asset{}
	for _, a := range want.Assets {
		wa[a.Name] = a
	}
	var errs []string
	for _, a := range got.Assets {
		x, ok := wa[a.Name]
		if !ok {
			errs = append(errs, a.Name+": not in the manifest")
			continue
		}
		if x.SHA256 != a.SHA256 {
			errs = append(errs, fmt.Sprintf("%s: sha256 %s, manifest %s", a.Name, a.SHA256, x.SHA256))
		}
		xf := map[string]File{}
		for _, f := range x.Files {
			xf[f.Path] = f
		}
		for _, f := range a.Files {
			y, ok := xf[f.Path]
			if !ok {
				errs = append(errs, fmt.Sprintf("%s/%s: not in the manifest", a.Name, f.Path))
				continue
			}
			fb, _ := json.Marshal(f)
			yb, _ := json.Marshal(y)
			if !bytes.Equal(fb, yb) {
				errs = append(errs, fmt.Sprintf("%s/%s: differs from the manifest (sha256 %s, manifest %s)", a.Name, f.Path, f.SHA256, y.SHA256))
			}
			delete(xf, f.Path)
		}
		for p := range xf {
			errs = append(errs, fmt.Sprintf("%s/%s: in the manifest but not in the asset", a.Name, p))
		}
		jb, _ := json.Marshal(a.Jar)
		kb, _ := json.Marshal(x.Jar)
		if !bytes.Equal(jb, kb) {
			errs = append(errs, a.Name+": JAR manifest values differ")
		}
	}
	if want.Go != got.Go {
		errs = append(errs, fmt.Sprintf("Go %s, manifest %s", got.Go, want.Go))
	}
	if len(errs) == 0 {
		errs = append(errs, "assets differ from the manifest (order or a missing asset)")
	}
	return fmt.Errorf("%d difference(s):\n  %s", len(errs), strings.Join(errs, "\n  "))
}

// origin — `go mod download -json`에서 쓰는 부분.
type origin struct {
	Version string
	Sum     string
	Origin  *struct {
		VCS  string
		URL  string
		Hash string
		Ref  string
	}
}

// downloadFunc — 모듈 출처를 얻는다. direct면 프록시를 거치지 않는다. 시험은 가짜를 넣는다.
type downloadFunc func(module, version string, direct bool) (*origin, error)

func goModDownload(module, version string, direct bool) (*origin, error) {
	dir, err := os.MkdirTemp("", "releasemanifest")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	// 빈 모듈 캐시를 따로 쓴다. 빌드가 채운 캐시의 정보에는 Origin.Ref가 빠져 있을 수 있다(모듈 그래프를 풀며
	// 받은 판은 태그 대신 다른 기록으로 남는다). 처음 받는 조회여야 프록시·저장소가 태그 ref를 준다.
	cache := filepath.Join(dir, "modcache")
	defer func() { // 읽기 전용 파일을 지우려면 go clean이 필요하다. 반드시 이 캐시만 가리킨다
		c := exec.Command("go", "clean", "-modcache")
		c.Env = append(os.Environ(), "GOFLAGS=", "GOMODCACHE="+cache)
		_ = c.Run()
	}()
	cmd := exec.Command("go", "mod", "download", "-json", module+"@"+version)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GO111MODULE=on", "GOFLAGS=", "GOMODCACHE="+cache)
	if direct {
		cmd.Env = append(cmd.Env, "GOPROXY=direct")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go mod download %s@%s: %v: %s", module, version, err, strings.TrimSpace(stderr.String()))
	}
	var o origin
	if err := json.Unmarshal(out, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

// repoCommits — 리포마다 태그 커밋을 모듈 출처에서 얻고 체크아웃 커밋과 대조한다. 확인하지 못하면 오류다.
func repoCommits(tag, checkouts string, dl downloadFunc) ([]Repo, error) {
	var out []Repo
	for _, name := range repos {
		mod := modPrefix + name
		local, err := gitCommit(filepath.Join(checkouts, name), tag)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		r, err := verifiedOrigin(mod, tag, local, dl)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		r.Name = name
		out = append(out, r)
	}
	return out, nil
}

// verifiedOrigin — 프록시의 모듈 출처, 없으면 직접 받은 출처. Ref가 그 태그이고 Hash가 체크아웃 커밋이어야 한다.
func verifiedOrigin(mod, tag, local string, dl downloadFunc) (Repo, error) {
	var why []string
	for _, direct := range []bool{false, true} {
		o, err := dl(mod, tag, direct)
		via := "proxy"
		if direct {
			via = "direct"
		}
		switch {
		case err != nil:
			why = append(why, via+": "+err.Error())
		case o.Origin == nil || o.Origin.Hash == "":
			why = append(why, via+": no Origin.Hash")
		case o.Origin.Ref != "refs/tags/"+tag:
			why = append(why, fmt.Sprintf("%s: Origin.Ref %q, want refs/tags/%s", via, o.Origin.Ref, tag))
		case o.Origin.Hash != local:
			return Repo{}, fmt.Errorf("module origin commit %s differs from the checkout's %s", o.Origin.Hash, local)
		case o.Sum == "":
			why = append(why, via+": no module sum")
		default:
			src := "module-origin"
			if direct {
				src = "module-origin-direct"
			}
			return Repo{Module: mod, Commit: o.Origin.Hash, OriginRef: o.Origin.Ref, Sum: o.Sum, Source: src}, nil
		}
	}
	return Repo{}, fmt.Errorf("the tag commit could not be verified from the module origin (%s)", strings.Join(why, "; "))
}

// gitCommit — 체크아웃의 HEAD 커밋. 릴리스 워크플로는 다섯 리포를 그 태그로 체크아웃한다(얕은 체크아웃에는 태그
// ref가 없을 수 있어 HEAD를 읽는다). 이 값과 모듈 출처의 커밋이 같아야 한다.
func gitCommit(dir, _ string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
