// Command checkdocs — 문서 게이트. md가 조용히 썩는 것을 막는다(`make check-docs`, `make` 전체에 포함).
//
// 검사하는 것:
//
//	(1) 링크 무결성   — 리포 안 상대 링크의 대상 파일이 있나, `#앵커`가 실제 제목인가.
//	(2) 낡은 범위 표현 — 이미 이 리포가 하는 일을 "범위 밖"이라 말하는 문장(구현이 앞서가고 문서가
//	    뒤처지면 사용자가 있는 기능을 없다고 읽는다 — 갭이 아니라 **오답**이다).
//	(3) 역할분담 산문  — 문서에는 **기능과 사용법**만 둔다. "실행은 사용자 몫" 같은 문장은 소프트웨어를
//	    쓰는 데 필요한 정보가 아니고, 손대는 사람마다 경계를 다시 쓰게 만든다.
//	(4) 개인정보      — 개발 머신 이름·홈 경로·계정이 공개 리포에 남지 않게.
//
// (2)와 (3)은 같은 뿌리다 — 이 리포는 **자기 밖을 설명하지 않는다.** 안 하는 일은 "하지 않는다"로
// 적고, 누가 하느냐는 적지 않는다. 계획은 로드맵이 말한다.
//
// ★ 왜 셸(grep)이 아닌가 — BSD grep은 `.`를 바이트로 세서 한글 문장에서 `{0,80}` 범위가 짧아진다.
// 같은 게이트가 Mac에서 통과하고 리눅스에서 실패했다. 로컬과 빌드 머신의 판정이 갈리면 게이트가 아니다.
// ★ 왜 Go인가 — 이 리포를 빌드하려면 Go가 이미 필요하다. 검사기 하나 때문에 다른 런타임을 요구하면
// 클론한 사람의 전제가 늘어난다(§2.3 최소 의존 — collector가 `ldd` 대신 ELF를 직접 읽는 것과 같은 결).
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	link     = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	head     = regexp.MustCompile(`(?m)^#{1,6}\s+(.*?)\s*$`)
	explicit = regexp.MustCompile(`<a\s+(?:id|name)="([^"]+)"`)

	// (2) **"이 리포 밖"이라는 위치 선언 자체를 금지한다.** 이 리포는 자기 밖을 설명하지 않는다 —
	//     안 하는 일은 "하지 않는다"로 적고, 계획은 로드맵이 말한다. (한때 120곳이 있었고, 그중
	//     상당수가 구현이 앞서가며 거짓이 됐다. 문구를 지운 뒤로는 아예 못 쓰게 막는 편이 싸다.)
	//     "범위 밖"만 단독으로 쓰는 경우(보안 신고 범위 등)는 다른 뜻이라 잡지 않는다.
	staleScope = regexp.MustCompile(`이 리포 밖|이 리포 범위 밖|리포 범위 밖`)

	// (3) 누가 하느냐를 설명하는 문장 — 그리고 그것을 표로 만든 것.
	//     유료 대행·벤더 서비스 축은 이 리포의 문서가 다룰 대상이 아니다. 산문만 잡던 초기
	//     규칙은 `| 활동 | 고객 | 소프트웨어 | 서비스 |` 같은 **표 머리**를 통과시켰다(실제로
	//     통과했다). '고객'·'벤더 서비스'는 어휘 자체를 막는다 — 읽는 사람은 사용자이지
	//     고객이 아니고, 유료 대행은 이 문서들의 관심사가 아니다.
	// §N · §1.5 · §1.5 같은 절 참조. 「§ 표기」 안내문 자신은 숫자가 안 붙어 걸리지 않는다.
	sectionRef = regexp.MustCompile(`§[0-9]+[0-9A-Za-z.]*`)
	roleProse  = regexp.MustCompile(`(사용자|고객|우리|도구)(의)? 몫|실행은 (사용자|고객)|(사용자|고객)가 실행한다|사용자 역할이다|고객|\[서비스\]|서비스 카탈로그|전문 서비스|벤더 인적 지원`)

	// (4) 개인 식별자만. `/home/me` 같은 일반 플레이스홀더는 문서에 필요한 예시다.
	// 머신 이름·경로·계정뿐 아니라 **내 개발 환경 구성**도 제품 문서에 남으면 안 된다 —
	// 리포를 클론하는 사람에게 "ubuntu-dev에서 빌드하라"는 말은 뜻이 없고, 내 환경이 바뀌면
	// 조용히 낡는다. 필요한 것은 "리눅스가 필요하다" 같은 **요건**이지 내 머신 이름이 아니다.
	// 패턴에는 **실제 식별자를 적지 않는다** — 개인정보를 막겠다는 규칙이 소스에 그것을
	// 적어두면 앞뒤가 안 맞는다. 모양만 잡는다: 홈 경로, 사용자@머신, 개발 머신 지칭.
	private = regexp.MustCompile(`/(?:home|Users)/([\w.-]+)|([\w.-]+)@([\w.-]*(?:desktop|laptop|pc))\b|([\w.-]+_desktop)\b|이 (?:Mac|맥|노트북)`)

	// 문서가 일부러 쓰는 일반 이름. 예시 경로에는 사람 이름이 아니라 이런 것이 들어간다.
	placeholders = map[string]bool{
		"me": true, "you": true, "user": true, "username": true, "example": true,
		"someone": true, "youruser": true, "myuser": true, "deploy": true,
	}

	// (5) 우리가 보는 것은 **암호가 아니라 알고리즘**이다 — 암호문도 키도 읽지 않는다.
	//     "무슨 암호를 쓰는지 관측한다"는 내용을 본다고 읽힌다. 반복해서 새어 들어와 게이트에 넣는다.
	imprecise = regexp.MustCompile(`(무슨|어떤) 암호를|암호를 (관측|읽|본다|봅니다)`)

	// slug 정리용 — Go의 `\w`는 ASCII 전용이라 한글이 날아간다. 문자·숫자는 스크립트 무관하게 남긴다.
	slugLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	slugMarkup = regexp.MustCompile("[`*_~]")
	slugDrop   = regexp.MustCompile(`[^\p{L}\p{N}_\s-]`)
	slugSpace  = regexp.MustCompile(`\s`)
	codeSpan   = regexp.MustCompile("`[^`]*`")
)

// tracked — git이 아는 파일만 본다(생성물·무시 대상은 검사하지 않는다).
func tracked(globs ...string) []string {
	args := append([]string{"-c", "core.quotePath=off", "ls-files"}, globs...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "git ls-files 실패:", err)
		os.Exit(1)
	}
	return strings.Fields(string(out))
}

// slug — GitHub 앵커 규칙 근사: 소문자화, 인라인 마크업·구두점 제거, 공백→하이픈.
// 한글은 그대로 남는다(그래서 국문 제목의 앵커도 검사할 수 있다).
func slug(text string) string {
	t := strings.ToLower(strings.TrimSpace(text))
	t = slugLink.ReplaceAllString(t, "$1") // 링크는 표시 문자열만
	t = slugMarkup.ReplaceAllString(t, "")
	t = slugDrop.ReplaceAllString(t, "")
	// GitHub은 공백을 **하나씩** 하이픈으로 바꾼다 — 합치면 연속 하이픈(`— ` 주변)이 어긋난다.
	return strings.Trim(slugSpace.ReplaceAllString(t, "-"), "-")
}

func lines(path string) ([]string, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	body := string(b)
	var out []string
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, body, sc.Err()
}

func main() {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "git 저장소가 아니다:", err)
		os.Exit(1)
	}
	if err := os.Chdir(strings.TrimSpace(string(root))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	docs := tracked("*.md")
	// 사용자에게 보이는 문구는 md에만 있지 않다 — 데모·예제 스크립트의 echo와 계약 .proto의 주석도
	// 같은 규칙을 받는다(링크 검사는 md만).
	prose := append(append([]string{}, docs...),
		tracked("demo/scripts/*.sh", "demo/scripts/internal/*.sh",
			"examples/**/*.sh", "contracts/proto/**/*.proto")...)

	anchors := map[string]map[string]bool{}
	bodies := map[string][]string{}
	for _, d := range docs {
		ls, body, err := lines(d)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		bodies[d] = ls
		set := map[string]bool{}
		for _, m := range head.FindAllStringSubmatch(body, -1) {
			set[slug(m[1])] = true
		}
		// 제목에서 만든 앵커 + 문서가 직접 박은 앵커(<a id="...">) 둘 다 유효하다.
		for _, m := range explicit.FindAllStringSubmatch(body, -1) {
			set[m[1]] = true
		}
		anchors[d] = set
	}

	titles := []string{
		`링크가 끊어졌다 — 문서를 옮기거나 제목을 바꿀 때 참조도 함께 고칠 것`,
		`이 리포가 하는 일을 "범위 밖"이라 말한다 — 구현을 따라 문장을 고칠 것`,
		`역할분담 산문 — 문서에는 기능·사용법만 둘 것`,
		`개인 개발 환경 정보가 남아 있다 — 일반화할 것(예: "빌드 머신")`,
		`부정확한 표현 — 우리가 보는 것은 암호가 아니라 **알고리즘**이다(암호문·키를 읽지 않는다)`,
		`제목만 있고 내용이 없는 절 — 옮기다 본문을 흘렸거나, 쓰다 만 자리다`,
		"범위 표기의 `~` — GitHub이 취소선으로 읽어 문장 일부에 줄이 그어진다. `–`를 쓸 것",
		"`§N`을 쓰면서 그게 어느 문서의 절인지 밝히지 않았다 — 처음 읽는 사람은 찾을 방법이 없다",
	}
	hits := make([][]string, len(titles))

	for _, d := range prose {
		ls, ok := bodies[d]
		if !ok {
			var err error
			if ls, _, err = lines(d); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		isDoc := anchors[d] != nil
		for n, line := range ls {
			if isDoc {
				for _, m := range link.FindAllStringSubmatch(line, -1) {
					target := m[1]
					if strings.HasPrefix(target, "http:") || strings.HasPrefix(target, "https:") ||
						strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
						continue
					}
					path, anchor, _ := strings.Cut(target, "#")
					if path == "" {
						continue
					}
					p := filepath.Clean(filepath.Join(filepath.Dir(d), path))
					if _, err := os.Stat(p); err != nil {
						hits[0] = append(hits[0], fmt.Sprintf("%s:%d: 링크 대상 없음 → %s", d, n+1, target))
					} else if anchor != "" && anchors[p] != nil && !anchors[p][slug(anchor)] {
						hits[0] = append(hits[0], fmt.Sprintf("%s:%d: 앵커 없음 → %s", d, n+1, target))
					}
				}
			}
			// 금지 문구는 한 줄 안에 있으므로 줄 단위로 본다 — 위치를 정확히 가리키기 위함.
			if staleScope.MatchString(line) {
				hits[1] = append(hits[1], loc(d, n, line))
			}
			if roleProse.MatchString(line) {
				hits[2] = append(hits[2], loc(d, n, line))
			}
			if isPrivate(line) {
				hits[3] = append(hits[3], loc(d, n, line))
			}
			if imprecise.MatchString(line) {
				hits[4] = append(hits[4], loc(d, n, line))
			}
		}
		// (6) 빈 절. 절을 옮기다 본문만 흘리면 제목이 남아 링크·앵커는 멀쩡하고 게이트도
		//     통과한다 — 표 하나가 통째로 사라져도 알 길이 없다. 여기서 막는다.
		if isDoc {
			hits[5] = append(hits[5], emptySections(d, ls)...)
			hits[6] = append(hits[6], tildeRanges(d, ls)...)
			hits[7] = append(hits[7], unexplainedSectionRefs(d, ls)...)
		}
	}

	// (8) 라이선스 표 ↔ 실제 링크되는 모듈. 손으로 관리하면 반드시 어긋난다(실제로 어긋났다).
	if dep := checkLicenseTable(); len(dep) > 0 {
		hits = append(hits, dep)
		titles = append(titles, "라이선스 문서가 실제 의존성과 어긋난다 — docs/라이선스_정리.md §2를 고칠 것")
	} else {
		hits = append(hits, nil)
		titles = append(titles, "")
	}

	fail := false
	for i, title := range titles {
		if title == "" {
			continue
		}
		if len(hits[i]) == 0 {
			continue
		}
		fail = true
		fmt.Printf("✗ %s\n", title)
		for _, h := range hits[i] {
			fmt.Printf("    %s\n", h)
		}
		fmt.Println()
	}
	if fail {
		fmt.Println("문서 게이트 실패 — 위 위치를 고치고 다시 `make check-docs`.")
		os.Exit(1)
	}
	fmt.Printf("✓ 문서 검사 통과 (%d개 md + %d개 스크립트·계약 — 링크·앵커·범위 표현·역할 산문·개인정보)\n",
		len(docs), len(prose)-len(docs))
}

// loc — 위치 + 문장 앞부분. 한글이 잘리지 않게 룬 단위로 자른다.
// isPrivate — 개인 식별자가 섞인 줄인가. 정규식은 **모양**만 잡고, 사람 이름 자리에
// 일반 플레이스홀더(`/home/me` 등)가 들어간 것은 통과시킨다. 실제 계정명을 이 소스에
// 적어두면 개인정보를 막겠다는 규칙이 스스로 그것을 남기는 꼴이 된다.
func isPrivate(line string) bool {
	for _, m := range private.FindAllStringSubmatch(line, -1) {
		named := false
		for _, g := range m[1:] {
			if g == "" {
				continue
			}
			named = true
			if !placeholders[strings.ToLower(g)] {
				return true
			}
		}
		if !named { // 이 Mac·이 맥 같은 지칭 — 잡을 그룹이 없다
			return true
		}
	}
	return false
}

func loc(file string, idx int, line string) string {
	s := strings.TrimSpace(line)
	if r := []rune(s); len(r) > 160 {
		s = string(r[:160])
	}
	return fmt.Sprintf("%s:%d: %s", file, idx+1, s)
}

// checkLicenseTable — 빌드 산출물에 실제로 링크되는 모듈(go list -deps)이 라이선스 문서의 표와
// 일치하나. 버전까지 본다 — 버전이 어긋나면 "어느 버전을 검토했나"가 거짓이 된다.
// 목록을 못 얻으면 그 사실을 실패로 낸다(조용히 통과시키지 않는다).
func checkLicenseTable() []string {
	const doc = "docs/라이선스_정리.md"
	b, err := os.ReadFile(doc)
	if err != nil {
		return nil // 문서가 없으면 이 검사 대상이 아니다
	}
	body := string(b)

	out, err := exec.Command("go", "list", "-deps",
		"-f", "{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}",
		"./discovery/cmd/...", "./inventory/cmd/...", "./provisioning/cmd/...").Output()
	if err != nil {
		// 환경 미비(gen/ 없음)를 문서 오류와 같은 실패로 다루지 않는다. 다만 **검사하지 못했다는
		// 사실**은 크게 남긴다 — `make`(전체)는 generate 뒤에 돌므로 거기서는 항상 검사된다.
		fmt.Fprintln(os.Stderr, "⚠ 라이선스 표 대조를 건너뛴다: `go list -deps` 실패(`make generate` 필요)")
		return nil
	}
	seen := map[string]bool{}
	var miss []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || strings.HasPrefix(f[0], "github.com/pqcota/") || seen[f[0]] {
			continue
		}
		seen[f[0]] = true
		mod, ver := f[0], f[1]
		switch {
		case !strings.Contains(body, mod):
			miss = append(miss, fmt.Sprintf("%s: `%s`가 표에 없다(링크되는데 고지 대상에서 빠졌다)", doc, mod))
		case !strings.Contains(body, ver):
			miss = append(miss, fmt.Sprintf("%s: `%s`의 버전이 다르다 — 실제 %s", doc, mod, ver))
		}
	}
	return miss
}

// unexplainedSectionRefs — `§2.5`·`§1.5` 같은 절 참조를 쓰면서 **그게 어느 문서의 절인지**
// 밝히지 않은 문서를 찾는다.
//
// 이 리포는 절 번호를 근거로 자주 든다. 쓰는 사람에겐 자명하지만 처음 읽는 사람에겐 찾을 방법이
// 없는 기호다 — 링크가 아니라 검색으로도 안 걸린다. 이미 문서 열 곳이 「§ 표기」 한 줄로 밝히고
// 있어, 관행은 서 있고 빠진 문서만 있는 상태였다. 그 한 줄을 게이트로 굳힌다.
//
// 규정서 자신은 면제한다 — 자기 절 번호를 자기가 가리키는 것이라 밝힐 대상이 없다.
// 영문 문서는 「§ notation」으로 같은 일을 한다.
func unexplainedSectionRefs(doc string, ls []string) []string {
	if strings.HasSuffix(doc, "플랫폼_규정.md") {
		return nil
	}
	body := strings.Join(ls, "\n")
	if strings.Contains(body, "§ 표기") || strings.Contains(body, "§ notation") {
		return nil
	}
	for n, line := range ls {
		if m := sectionRef.FindString(line); m != "" {
			return []string{fmt.Sprintf("%s:%d: %s — 「§ 표기」 한 줄로 어느 문서의 절인지 밝힐 것", doc, n+1, m)}
		}
	}
	return nil
}

// emptySections — 제목 다음에 본문도 하위 제목도 없는 절을 찾는다.
//
// `## 묶음` 바로 밑에 `### 항목`이 오는 것은 정상이다 — 그 절의 내용이 하위 절들이다.
// 문제는 제목 다음에 **아무것도 없이 같은 레벨(또는 더 얕은) 제목이 오는 것**이다. 절을
// 옮기다 본문만 흘리면 그 모양이 되는데, 링크·앵커는 멀쩡해 다른 규칙에 걸리지 않는다.
// 표 하나가 통째로 사라져도 알 길이 없다.
func emptySections(doc string, ls []string) []string {
	level := func(line string) int {
		n := len(line) - len(strings.TrimLeft(line, "#"))
		if n == 0 || n > 6 || !strings.HasPrefix(line[n:], " ") {
			return 0 // 제목이 아니다(코드 블록의 주석 등)
		}
		return n
	}
	var out []string
	head, headLv, title := -1, 0, ""
	flush := func(end, nextLv int) {
		if head < 0 {
			return
		}
		if nextLv > headLv && nextLv != 0 {
			return // 하위 제목이 이어진다 — 그 절의 내용이 하위 절들이다
		}
		for i := head + 1; i < end; i++ {
			if strings.TrimSpace(ls[i]) != "" {
				return
			}
		}
		out = append(out, fmt.Sprintf("%s:%d: %s", doc, head+1, strings.TrimSpace(title)))
	}
	inFence := false
	for n, line := range ls {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lv := level(line)
		if lv == 0 {
			continue
		}
		flush(n, lv)
		head, headLv, title = n, lv, line
	}
	flush(len(ls), 0)
	return out
}

// tildeRanges — 산문에서 범위 구분자로 쓴 `~`를 찾는다.
//
// GFM에서 `~`는 취소선 문법이라, 한 문단에 `S1~S7`처럼 두 번 나오면 첫 `~`가 열고 둘째가
// 닫아 **그 사이 문장 전체에 줄이 그어진다**. 줄을 넘어도 한 문단이면 짝이 맞으므로
// 줄 단위로는 안 보인다 — 문단 전체가 조용히 그어진다.
// 짝이 맞는 `~~취소선~~`은 의도한 것이라 건드리지 않는다.
func tildeRanges(doc string, ls []string) []string {
	var out []string
	inFence := false
	for n, line := range ls {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		bare := codeSpan.ReplaceAllString(line, "") // 백틱 안은 그대로 나간다
		bare = strings.ReplaceAll(bare, "~~", "")   // 의도한 취소선 제외
		if strings.Contains(bare, "~") {
			out = append(out, loc(doc, n, line))
		}
	}
	return out
}
