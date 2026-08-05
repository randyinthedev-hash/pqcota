// Command topogen — demo/topology/topology.yaml → 데모 산출물 생성기.
//
// 한 명세로 네 산출물을 낸다(데모가 네 곳에 하드코딩하던 것):
//
//	docker-compose.yml  서비스(노드별 build target·args·networks) + ctl + pg
//	manifest.env        bash 소스용 — NODES 배열 · EDGE_COUNT · 사람이 읽는 이름(HUMAN)
//	groups.ini          Ansible 그룹(openssl/java) + 엣지→traffic 시나리오
//	profiles.csv        CMDB 프로필(pqcota-profile 형식)
//
// ★ 정직성(§2.6): 지원 못 하는 조합(관측 불가 종류·s_server 없는 fork)은 조용히 깨진 노드를
// 내지 않고 **명확한 오류로 거부**한다.
//
// Go인 이유 — 리포의 다른 실행물과 같은 툴체인을 쓴다(외부 런타임 불요, `go test`로 회귀를
// 막는다). 데모 전용이라 제품 바이너리는 아니고, up.sh가 컨테이너 안에서 돌린다.
//
// usage: topogen <topology.yaml> <out-dir>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── 입력 명세 ──────────────────────────────────────────────────────────────

type Spec struct {
	Networks []string `yaml:"networks"`
	Nodes    []Node   `yaml:"nodes"`
	Edges    []Edge   `yaml:"edges"`
}

type Node struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Kind     string   `yaml:"kind"` // openssl | java
	Role     string   `yaml:"role"` // openssl: client | server
	OpenSSL  *OpenSSL `yaml:"openssl"`
	JCA      *JCA     `yaml:"jca"`
	Apps     []string `yaml:"apps"` // openssl server: 한 libssl을 로드하는 앱들(다중 귀속)
	Networks []string `yaml:"networks"`
	Profile  *Profile `yaml:"profile"`
}

type OpenSSL struct {
	Fork    string  `yaml:"fork"`
	Version flexStr `yaml:"version"` // "1.1.1"·"3.0"·3 — 따옴표 유무에 관대하게
}

type JCA struct {
	Providers []string `yaml:"providers"`
}

type Profile struct {
	Env   string `yaml:"env"`
	Role  string `yaml:"role"`
	Owner string `yaml:"owner"`
}

type Edge struct {
	From  string `yaml:"from"`
	To    string `yaml:"to"`
	Proto string `yaml:"proto"` // pqc | ssl | ssh
	Port  int    `yaml:"port"`
}

// flexStr — 스칼라를 적힌 그대로 문자열로 받는다. `version: 3.0`(따옴표 없음)을 float로 읽어
// "cannot unmarshal !!float" 같은 불친절한 오류를 내지 않기 위함.
type flexStr string

func (f *flexStr) UnmarshalYAML(n *yaml.Node) error { *f = flexStr(n.Value); return nil }

// ── 지원 매트릭스 ──────────────────────────────────────────────────────────

// opensslBase — fork=openssl일 때 version → base 이미지(그 배포판의 OpenSSL 버전).
var opensslBase = map[string]string{
	"1.1.1": "ubuntu:20.04", // OpenSSL 1.1.1 (레거시·양자취약)
	"3.0":   "ubuntu:22.04", // OpenSSL 3.0
	"3":     "ubuntu:24.04", // OpenSSL 3.x
	"3.x":   "ubuntu:24.04",
}

// forkTarget — fork → Dockerfile target.
var forkTarget = map[string]string{"openssl": "node-openssl", "libressl": "node-libressl"}

// forkUnsupported — 데모 서버 노드로 못 띄우는 fork를 이유와 함께 거부한다(추측해서 깨진 노드 안 냄).
var forkUnsupported = map[string]string{
	"boringssl": "BoringSSL은 s_server(-www)가 없어 데모 서버 노드로 못 띄운다",
	"aws-lc":    "AWS-LC는 데모용 s_server 상당 도구가 없다",
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// buildContext — 생성된 compose 파일(demo/.generated/docker-compose.yml)에서 리포 루트까지.
// ★ compose는 build context를 **compose 파일 위치 기준**으로 푼다 — 산출물 경로가 바뀌면
// 이 값도 함께 바뀌어야 한다 — 안 맞추면 리포 밖을 가리킨다. TestBuildContext가 못 박는다.
const buildContext = "../.."

// ── 검증 ───────────────────────────────────────────────────────────────────

// Validate — 명세의 구조·지원 여부를 확인한다. 모호하면 통과시키지 않고 이유를 붙여 거부한다.
func Validate(s *Spec) error {
	if len(s.Nodes) == 0 {
		return fmt.Errorf("nodes가 비었다 — 최소 1개 노드가 필요하다")
	}
	nets := map[string]bool{}
	for _, n := range s.Networks {
		nets[n] = true
	}
	ids := map[string]bool{}
	for _, n := range s.Nodes {
		if !idRe.MatchString(n.ID) {
			return fmt.Errorf("노드 id는 소문자/숫자/-만: %q", n.ID)
		}
		if ids[n.ID] {
			return fmt.Errorf("노드 id 중복: %s", n.ID)
		}
		ids[n.ID] = true

		switch n.Kind {
		case "openssl":
			fork := n.forkName()
			if why, bad := forkUnsupported[fork]; bad {
				return fmt.Errorf("%s: fork=%s 미지원 — %s", n.ID, fork, why)
			}
			if _, ok := forkTarget[fork]; !ok {
				return fmt.Errorf("%s: 알 수 없는 fork %q (openssl|libressl)", n.ID, fork)
			}
			if fork == "openssl" {
				if _, ok := opensslBase[n.version()]; !ok {
					return fmt.Errorf("%s: openssl version %q 미지원 — %v 중 하나", n.ID, n.version(), sortedKeys(opensslBase))
				}
			}
			if r := n.roleName(); r != "client" && r != "server" {
				return fmt.Errorf("%s: openssl role은 client|server. 받은 값 %q", n.ID, r)
			}
		case "java":
		default:
			return fmt.Errorf("%s: kind는 openssl|java만(pqcota가 관측하는 것만). 받은 값 %q", n.ID, n.Kind)
		}

		for _, net := range n.Networks {
			if len(nets) > 0 && !nets[net] {
				return fmt.Errorf("%s: 네트워크 %q가 networks에 없다", n.ID, net)
			}
		}
	}
	for _, e := range s.Edges {
		if !ids[e.From] {
			return fmt.Errorf("엣지의 from=%q가 노드에 없다", e.From)
		}
		if !ids[e.To] {
			return fmt.Errorf("엣지의 to=%q가 노드에 없다", e.To)
		}
		switch e.Proto {
		case "pqc", "ssl", "ssh":
		default:
			return fmt.Errorf("엣지 proto는 pqc|ssl|ssh. 받은 값 %q", e.Proto)
		}
	}
	return nil
}

func (n Node) forkName() string {
	if n.OpenSSL != nil && n.OpenSSL.Fork != "" {
		return n.OpenSSL.Fork
	}
	return "openssl"
}

func (n Node) version() string {
	if n.OpenSSL != nil && n.OpenSSL.Version != "" {
		return string(n.OpenSSL.Version)
	}
	return "3"
}

func (n Node) roleName() string {
	if n.Role != "" {
		return n.Role
	}
	return "client"
}

func (n Node) displayName() string {
	if n.Name != "" {
		return n.Name
	}
	return n.ID
}

// ── 산출물 ─────────────────────────────────────────────────────────────────

// serverRoles — 이 노드가 어떤 서버를 띄워야 하나. 엣지의 대상(to)이 되는 proto로 결정한다.
func serverRoles(id string, edges []Edge) (pqc, ssl bool) {
	for _, e := range edges {
		if e.To != id {
			continue
		}
		switch e.Proto {
		case "pqc":
			pqc = true
		case "ssl":
			ssl = true
		}
	}
	return
}

// nodeEnv — 노드 엔트리포인트가 읽는 역할 환경변수(순서 고정 — 결정론적 출력).
func nodeEnv(n Node, edges []Edge) [][2]string {
	env := [][2]string{{"NODE_NAME", n.ID}}
	pqc, ssl := serverRoles(n.ID, edges)
	if n.Kind == "java" {
		env = append(env, [2]string{"JAVA_APP", "1"})
		var provs []string
		if n.JCA != nil {
			provs = n.JCA.Providers
		}
		env = append(env, [2]string{"PQCOTA_PROVIDERS", strings.Join(provs, ",")})
	}
	if pqc {
		env = append(env, [2]string{"PQC_SERVER", "1"})
	}
	if ssl || (n.Kind == "openssl" && n.roleName() == "server") {
		apps := n.Apps
		if len(apps) == 0 {
			apps = []string{n.ID} // 기본=단일 앱(node id). 목록을 주면 공유 .so 다중 귀속.
		}
		env = append(env, [2]string{"SSL_SERVER", "1"}, [2]string{"SSL_APPS", strings.Join(apps, ",")})
	}
	return env
}

// Compose — docker-compose.yml 텍스트. 직접 쓴다(마샬러의 맵 순서 비결정성을 피하고,
// 플레이북 생성기(pkg/provisioning)와 같은 방식으로 출력을 완전히 통제한다).
func Compose(s *Spec) string {
	nets := s.Networks
	if len(nets) == 0 {
		nets = []string{"pqcota-demo"}
	}
	var b strings.Builder
	b.WriteString("# 생성됨: topogen (demo/topology/topology.yaml) — 직접 고치지 말 것.\n")
	b.WriteString("# pqcota-ctl(컨트롤러)·pqcota-demo-pg(인벤토리 저장소)는 명세에 없고 여기서 자동으로 붙는다 —\n")
	b.WriteString("# 명세는 '관측 대상'을 적는 곳이라. 둘은 컨트롤러가 모든 노드에 닿도록 전 세그먼트에 참여한다.\n")
	b.WriteString("name: pqcota-demo-topo\n\nvolumes:\n  pqcota-gocache:\n  pqcota-gomod:\n\nnetworks:\n")
	for _, n := range nets {
		fmt.Fprintf(&b, "  %s:\n    name: pqcota-topo-%s\n    driver: bridge\n", n, n)
	}
	b.WriteString("\nservices:\n")

	// 컨트롤러 + Postgres — 모든 세그먼트에 붙여 어디서든 접근·적재 가능하게.
	writeSvc(&b, "pqcota-ctl", func(sb *strings.Builder) {
		fmt.Fprintf(sb, "    build: { context: %s, dockerfile: demo/Dockerfile, target: ctl }\n", buildContext)
		// ctl은 **빌드 머신**이라 Go 캐시를 볼륨에 둔다 — 두 번째 up.sh부터 컴파일이 짧아진다.
		sb.WriteString("    volumes: [pqcota-gocache:/root/.cache/go-build, pqcota-gomod:/root/go/pkg/mod]\n")
		sb.WriteString("    image: pqcota-demo/ctl\n    container_name: pqcota-ctl\n    hostname: pqcota-ctl\n")
		fmt.Fprintf(sb, "    networks: [%s]\n", strings.Join(nets, ", "))
		sb.WriteString("    labels: { pqcota-demo: \"1\" }\n")
	})
	writeSvc(&b, "pqcota-demo-pg", func(sb *strings.Builder) {
		sb.WriteString("    image: postgres:16\n    container_name: pqcota-demo-pg\n    hostname: pqcota-demo-pg\n")
		sb.WriteString("    environment: { POSTGRES_DB: pqcota, POSTGRES_PASSWORD: pqcota }\n")
		fmt.Fprintf(sb, "    networks: [%s]\n", strings.Join(nets, ", "))
		sb.WriteString("    labels: { pqcota-demo: \"1\" }\n")
	})

	for _, n := range s.Nodes {
		target := "node-java"
		var args string
		if n.Kind == "openssl" {
			target = forkTarget[n.forkName()]
			if n.forkName() == "openssl" {
				args = fmt.Sprintf(", args: { OPENSSL_BASE: \"%s\" }", opensslBase[n.version()])
			}
		}
		nn := n.Networks
		if len(nn) == 0 {
			nn = nets
		}
		node := n
		writeSvc(&b, n.ID, func(sb *strings.Builder) {
			fmt.Fprintf(sb, "    build: { context: %s, dockerfile: demo/Dockerfile, target: %s%s }\n", buildContext, target, args)
			fmt.Fprintf(sb, "    image: pqcota-demo/%s\n    container_name: %s\n    hostname: %s\n", node.ID, node.ID, node.ID)
			sb.WriteString("    environment:\n")
			for _, kv := range nodeEnv(node, s.Edges) {
				fmt.Fprintf(sb, "      %s: \"%s\"\n", kv[0], kv[1])
			}
			sb.WriteString("    cap_add: [NET_RAW]\n")
			fmt.Fprintf(sb, "    networks: [%s]\n", strings.Join(nn, ", "))
			sb.WriteString("    labels: { pqcota-demo: \"1\" }\n")
		})
	}
	return b.String()
}

func writeSvc(b *strings.Builder, name string, body func(*strings.Builder)) {
	fmt.Fprintf(b, "  %s:\n", name)
	body(b)
}

// GroupsINI — Ansible 그룹 멤버십 + 트래픽 시나리오(접속 정보는 targets.ini에 있다).
func GroupsINI(s *Spec) string {
	bySrc := map[string][]string{}
	for _, e := range s.Edges {
		port := e.Port
		if port == 0 {
			if e.Proto == "ssh" {
				port = 22
			} else {
				port = 4433
			}
		}
		bySrc[e.From] = append(bySrc[e.From], fmt.Sprintf("%s:%s:%d", e.Proto, e.To, port))
	}
	var b strings.Builder
	b.WriteString("# 생성됨: topogen — 그룹 멤버십 + 트래픽 시나리오(접속 정보 아님).\n")
	b.WriteString("# 접속(host·user·key)은 pqcota-hosts가 hosts.csv에서 만드는 targets.ini에.\n\n")
	for _, kind := range []string{"openssl", "java"} {
		var members []Node
		for _, n := range s.Nodes {
			if n.Kind == kind {
				members = append(members, n)
			}
		}
		if len(members) == 0 {
			continue
		}
		fmt.Fprintf(&b, "[%s]\n", kind)
		for _, n := range members {
			fmt.Fprintf(&b, "%s traffic=\"%s\"\n", n.ID, strings.Join(bySrc[n.ID], " "))
		}
		b.WriteString("\n")
	}
	b.WriteString("[targets:children]\nopenssl\njava\n\n")
	b.WriteString("[all:vars]\nwindow=16\nansible_python_interpreter=/usr/bin/python3\n")
	return b.String()
}

// ProfilesCSV — pqcota-profile 형식(demo.sh가 쓰는 헤더와 동일해야 한다).
func ProfilesCSV(s *Spec) string {
	var b strings.Builder
	b.WriteString("node_id,display_name,environment,role,owner\n")
	for _, n := range s.Nodes {
		var env, role, owner string
		if n.Profile != nil {
			env, role, owner = n.Profile.Env, n.Profile.Role, n.Profile.Owner
		}
		fmt.Fprintf(&b, "%s,%s,%s,%s,%s\n", n.ID, n.displayName(), env, role, owner)
	}
	return b.String()
}

// ManifestEnv — up.sh/demo.sh가 source해 바로 쓰는 bash 선언.
func ManifestEnv(s *Spec) string {
	ids := make([]string, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		ids = append(ids, n.ID)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "NODES=(%s)\n", strings.Join(ids, " "))
	fmt.Fprintf(&b, "EDGE_COUNT=%d\n", len(s.Edges))
	b.WriteString("declare -A HUMAN=(\n")
	for _, n := range s.Nodes {
		fmt.Fprintf(&b, "  [%s]=\"%s\"\n", n.ID, n.displayName())
	}
	b.WriteString(")\n")
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: topogen <topology.yaml> <out-dir>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "토폴로지 읽기:", err)
		os.Exit(1)
	}
	var spec Spec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		fmt.Fprintln(os.Stderr, "토폴로지 파싱:", err)
		os.Exit(1)
	}
	if err := Validate(&spec); err != nil {
		fmt.Fprintln(os.Stderr, "토폴로지 오류:", err)
		os.Exit(1)
	}
	out := os.Args[2]
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "출력 디렉터리:", err)
		os.Exit(1)
	}
	files := map[string]string{
		"docker-compose.yml": Compose(&spec),
		"groups.ini":         GroupsINI(&spec),
		"profiles.csv":       ProfilesCSV(&spec),
		"manifest.env":       ManifestEnv(&spec),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "쓰기:", err)
			os.Exit(1)
		}
	}
	nets := len(spec.Networks)
	if nets == 0 {
		nets = 1
	}
	fmt.Printf("생성 완료 → %s (노드 %d, 네트워크 %d, 엣지 %d)\n", out, len(spec.Nodes), nets, len(spec.Edges))
}
