package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parse(t *testing.T, src string) *Spec {
	t.Helper()
	var s Spec
	if err := yaml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatalf("파싱: %v", err)
	}
	return &s
}

const sample = `
networks: [corp, db]
nodes:
  - id: web-gw
    name: 결제 웹
    kind: openssl
    role: client
    openssl: { fork: openssl, version: "3.0" }
    networks: [corp]
    profile: { env: production, role: web, owner: 플랫폼팀 }
  - id: pay-app
    kind: java
    jca: { providers: [BC] }
    networks: [corp]
  - id: pay-db
    kind: openssl
    role: server
    openssl: { fork: openssl, version: "1.1.1" }
    apps: [payment-gw, api-gw]
    networks: [corp, db]
edges:
  - { from: web-gw, to: pay-app, proto: pqc, port: 8443 }
  - { from: web-gw, to: pay-db, proto: ssl, port: 4433 }
`

// 생성물 위치(demo/.generated/)를 옮기면 build context도 함께 고쳐야 한다 — 안 그러면
// 리포 '바깥'을 가리킨다. compose는 context를 **compose 파일 기준**으로 푼다.
func TestBuildContext(t *testing.T) {
	if buildContext != "../.." {
		t.Fatalf("compose가 demo/.generated/에 놓이므로 리포 루트는 ../.. 다 (받은 값 %q)", buildContext)
	}
	out := Compose(parse(t, sample))
	if strings.Contains(out, "context: ../../..") {
		t.Errorf("context가 리포 밖을 가리킨다:\n%s", out)
	}
	if !strings.Contains(out, "context: ../..") {
		t.Errorf("build context가 없다:\n%s", out)
	}
}

// 도구 쪽 컨테이너(ctl·pg)는 명세에 없고 생성기가 붙인다 — 명세는 '관측 대상'만 적기 때문.
// 둘은 컨트롤러가 어느 노드에든 SSH로 닿도록 **모든 세그먼트**에 참여해야 한다.
func TestToolContainersInjected(t *testing.T) {
	s := parse(t, sample)
	out := Compose(s)
	for _, want := range []string{"  pqcota-ctl:", "  pqcota-demo-pg:"} {
		if !strings.Contains(out, want) {
			t.Errorf("도구 컨테이너 %q가 주입되지 않았다:\n%s", want, out)
		}
	}
	// 명세엔 없어야 한다(사용자가 지워서 데모를 망가뜨릴 여지를 두지 않는다).
	for _, n := range s.Nodes {
		if n.ID == "pqcota-ctl" || n.ID == "pqcota-demo-pg" {
			t.Errorf("도구 컨테이너가 명세 노드로 들어왔다: %s", n.ID)
		}
	}
	// 전 세그먼트 참여 — 샘플은 corp·db 둘.
	if strings.Count(out, "networks: [corp, db]") < 2 {
		t.Errorf("ctl·pg가 모든 세그먼트에 붙어야 한다:\n%s", out)
	}
}

// 노드 종류·fork·version → Dockerfile target/base 이미지 매핑.
func TestComposeTargets(t *testing.T) {
	out := Compose(parse(t, sample))
	for _, want := range []string{
		`target: node-openssl, args: { OPENSSL_BASE: "ubuntu:22.04" }`, // web-gw 3.0
		`target: node-openssl, args: { OPENSSL_BASE: "ubuntu:20.04" }`, // pay-db 1.1.1
		"target: node-java",    // pay-app
		"networks: [corp, db]", // pay-db 다중 세그먼트
	} {
		if !strings.Contains(out, want) {
			t.Errorf("compose에 %q 없음:\n%s", want, out)
		}
	}
}

// 서버 역할은 엣지의 대상(to)에서 유도된다 — pqc 대상은 PQC 서버, ssl 대상·role:server는 s_server.
func TestNodeEnvDerivedFromEdges(t *testing.T) {
	s := parse(t, sample)
	env := func(id string) map[string]string {
		m := map[string]string{}
		for _, n := range s.Nodes {
			if n.ID == id {
				for _, kv := range nodeEnv(n, s.Edges) {
					m[kv[0]] = kv[1]
				}
			}
		}
		return m
	}
	if e := env("pay-app"); e["PQC_SERVER"] != "1" || e["JAVA_APP"] != "1" || e["PQCOTA_PROVIDERS"] != "BC" {
		t.Errorf("pay-app: pqc 엣지 대상 + java + BC여야: %v", e)
	}
	if e := env("pay-db"); e["SSL_SERVER"] != "1" || e["SSL_APPS"] != "payment-gw,api-gw" {
		t.Errorf("pay-db: 공유 .so 다중 앱이어야: %v", e)
	}
	if e := env("web-gw"); e["SSL_SERVER"] == "1" || e["PQC_SERVER"] == "1" {
		t.Errorf("web-gw는 클라이언트라 서버가 없어야: %v", e)
	}
}

// 트래픽 시나리오는 소스 노드에 붙는다(관측 창을 그 노드에서 채운다).
func TestGroupsTraffic(t *testing.T) {
	out := GroupsINI(parse(t, sample))
	if !strings.Contains(out, `web-gw traffic="pqc:pay-app:8443 ssl:pay-db:4433"`) {
		t.Errorf("소스 노드에 엣지가 안 붙었다:\n%s", out)
	}
	if !strings.Contains(out, `pay-db traffic=""`) {
		t.Errorf("대상 전용 노드는 빈 traffic이어야:\n%s", out)
	}
}

// profiles.csv 헤더는 pqcota-profile(그리고 demo.sh)이 기대하는 것과 같아야 한다.
func TestProfilesHeader(t *testing.T) {
	out := ProfilesCSV(parse(t, sample))
	if !strings.HasPrefix(out, "node_id,display_name,environment,role,owner\n") {
		t.Errorf("헤더 불일치:\n%s", out)
	}
	if !strings.Contains(out, "web-gw,결제 웹,production,web,플랫폼팀") {
		t.Errorf("프로필 행이 없다:\n%s", out)
	}
	if !strings.Contains(out, "pay-app,pay-app,,,") { // name·profile 생략 시 id로 폴백
		t.Errorf("생략 시 폴백이 없다:\n%s", out)
	}
}

// manifest는 bash가 source해 그대로 쓴다(배열·연관배열 문법).
func TestManifest(t *testing.T) {
	out := ManifestEnv(parse(t, sample))
	for _, want := range []string{"NODES=(web-gw pay-app pay-db)", "EDGE_COUNT=2", `[web-gw]="결제 웹"`} {
		if !strings.Contains(out, want) {
			t.Errorf("manifest에 %q 없음:\n%s", want, out)
		}
	}
}

// ★ 정직성(§2.5) — 관측 못 하는 종류·못 띄우는 fork는 조용히 넘어가지 않고 거부한다.
func TestValidateRejects(t *testing.T) {
	cases := map[string]string{
		"관측 못 하는 종류":       "nodes:\n  - {id: a, kind: dotnet}\n",
		"s_server 없는 fork": "nodes:\n  - {id: a, kind: openssl, openssl: {fork: boringssl}}\n",
		"미지원 version":      "nodes:\n  - {id: a, kind: openssl, openssl: {fork: openssl, version: \"0.9\"}}\n",
		"잘못된 id":           "nodes:\n  - {id: Web_GW, kind: java}\n",
		"id 중복":            "nodes:\n  - {id: a, kind: java}\n  - {id: a, kind: java}\n",
		"없는 노드로 가는 엣지":     "nodes:\n  - {id: a, kind: java}\nedges:\n  - {from: a, to: ghost, proto: ssl}\n",
		"미지원 proto":        "nodes:\n  - {id: a, kind: java}\nedges:\n  - {from: a, to: a, proto: quic}\n",
		"선언 안 한 네트워크":      "networks: [corp]\nnodes:\n  - {id: a, kind: java, networks: [nope]}\n",
		"빈 nodes":          "networks: [corp]\n",
	}
	for name, src := range cases {
		if err := Validate(parse(t, src)); err == nil {
			t.Errorf("%s: 거부해야 한다", name)
		}
	}
	// 정상 명세는 통과.
	if err := Validate(parse(t, sample)); err != nil {
		t.Errorf("정상 명세를 거부했다: %v", err)
	}
}

// 따옴표 없는 version(3.0 → float)도 적힌 그대로 읽어야 한다 — 불친절한 파싱 오류 방지.
func TestUnquotedVersion(t *testing.T) {
	s := parse(t, "nodes:\n  - {id: a, kind: openssl, openssl: {fork: openssl, version: 3.0}}\n")
	if got := s.Nodes[0].version(); got != "3.0" {
		t.Errorf("따옴표 없는 version을 %q로 읽었다 — 적힌 그대로여야", got)
	}
	if err := Validate(s); err != nil {
		t.Errorf("3.0은 지원 버전이다: %v", err)
	}
}
