// Command checkcollectors — **노드에 올릴 collector 목록이 두 곳에서 갈라지지 않게** 막는다.
//
// 같은 목록이 릴리스 워크플로와 참조 플레이북 양쪽에 있다:
//
//	.github/workflows/release.yml   무엇을 빌드해 번들에 넣나
//	discovery/ansible/discover.yml  무엇을 노드로 반입하나
//
// 한쪽만 바뀌면 **받아서 돌릴 때** 드러난다 — v0.6.3에서 플레이북에 `pqcota-jvmscan`을 더하고
// 워크플로를 안 고쳐, 릴리스만 받은 사람은 반입 단계에서 실패했다(v0.6.6에서 고침). 사람 기억에
// 맡기지 않고 빌드에서 막는다. checkdocs와 같은 이유로 Go다 — 새 런타임 전제가 없다.
package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	workflow = ".github/workflows/release.yml"
	playbook = "discovery/ansible/discover.yml"
)

func main() {
	wf, err := os.ReadFile(workflow)
	if err != nil {
		fail(err)
	}
	pb, err := os.ReadFile(playbook)
	if err != nil {
		fail(err)
	}

	built := builtByOS(string(wf))
	shipped, err := shippedByOS(pb)
	if err != nil {
		fail(err)
	}

	bad := false
	for _, os_ := range []string{"linux", "windows"} {
		b, s := built[os_], shipped[os_]
		if len(b) == 0 || len(s) == 0 {
			fmt.Printf("✗ %s: nothing was found — the shape of one of the two files changed, so this check is now blind\n", os_)
			fmt.Printf("    built in %s: %v\n    deployed by %s: %v\n", workflow, b, playbook, s)
			bad = true
			continue
		}
		if diff := symmetricDiff(b, s); len(diff) > 0 {
			fmt.Printf("✗ %s: the two lists disagree on %v\n", os_, diff)
			fmt.Printf("    built in %s      : %v\n", workflow, b)
			fmt.Printf("    deployed by %s : %v\n", playbook, s)
			fmt.Printf("    a collector the playbook deploys but the release does not build fails at deployment\n")
			fmt.Printf("    for whoever runs from the release bundle.\n")
			bad = true
		}
	}
	if bad {
		os.Exit(1)
	}
	fmt.Printf("✓ collector lists agree (linux %v · windows %v)\n", built["linux"], built["windows"])
}

// buildStep — `GOOS=<os>` 빌드 한 덩이. run 블록 안의 셸이라 YAML로는 못 읽고 텍스트로 본다.
var buildStep = regexp.MustCompile(`GOOS=(linux|windows)[^\n]*go build[^\n]*(?:\n[^\n]*)*?(?:\n\s*\n|\n\s*[a-z(])`)
var cmdPath = regexp.MustCompile(`\./discovery/cmd/(pqcota-[a-z]+)`)

// builtByOS — 워크플로가 OS별로 무엇을 빌드하나.
func builtByOS(s string) map[string][]string {
	out := map[string][]string{}
	for _, m := range buildStep.FindAllStringSubmatch(s, -1) {
		for _, p := range cmdPath.FindAllStringSubmatch(m[0], -1) {
			out[m[1]] = appendOnce(out[m[1]], p[1])
		}
	}
	return out
}

// shippedByOS — 플레이북이 OS별로 무엇을 반입하나. 블록의 when 절이 OS를 정하고, 그 안의
// loop가 반입 목록이다. `.exe`는 떼고 견준다(같은 collector의 Windows 이름일 뿐이다).
func shippedByOS(b []byte) (map[string][]string, error) {
	var plays []struct {
		Tasks []struct {
			When  string `yaml:"when"`
			Block []struct {
				Loop []string `yaml:"loop"`
			} `yaml:"block"`
		} `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(b, &plays); err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, p := range plays {
		for _, t := range p.Tasks {
			os_ := "linux"
			if strings.Contains(t.When, "== 'Windows'") {
				os_ = "windows"
			} else if !strings.Contains(t.When, "!= 'Windows'") {
				continue // OS로 가르는 블록이 아니다
			}
			for _, inner := range t.Block {
				for _, item := range inner.Loop {
					if name := strings.TrimSuffix(item, ".exe"); strings.HasPrefix(name, "pqcota-") {
						out[os_] = appendOnce(out[os_], name)
					}
				}
			}
		}
	}
	return out, nil
}

func appendOnce(xs []string, x string) []string {
	for _, e := range xs {
		if e == x {
			return xs
		}
	}
	xs = append(xs, x)
	sort.Strings(xs)
	return xs
}

// symmetricDiff — 한쪽에만 있는 것들. 어느 쪽에 없는지는 호출부가 두 목록을 함께 찍어 보인다.
func symmetricDiff(a, b []string) []string {
	in := func(xs []string, x string) bool {
		for _, e := range xs {
			if e == x {
				return true
			}
		}
		return false
	}
	var out []string
	for _, x := range a {
		if !in(b, x) {
			out = appendOnce(out, x)
		}
	}
	for _, x := range b {
		if !in(a, x) {
			out = appendOnce(out, x)
		}
	}
	return out
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "checkcollectors:", err)
	os.Exit(2)
}
