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
		t.Fatalf("parse: %v", err)
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
		t.Fatalf("compose lands in demo/.generated/, so the repo root is ../.. (got %q)", buildContext)
	}
	out := Compose(parse(t, sample))
	if strings.Contains(out, "context: ../../..") {
		t.Errorf("the build context points outside the repo:\n%s", out)
	}
	if !strings.Contains(out, "context: ../..") {
		t.Errorf("no build context:\n%s", out)
	}
}

// 도구 쪽 컨테이너(ctl·pg)는 명세에 없고 생성기가 붙인다 — 명세는 '관측 대상'만 적기 때문.
// 둘은 컨트롤러가 어느 노드에든 SSH로 닿도록 **모든 세그먼트**에 참여해야 한다.
func TestToolContainersInjected(t *testing.T) {
	s := parse(t, sample)
	out := Compose(s)
	for _, want := range []string{"  pqcota-ctl:", "  pqcota-demo-pg:"} {
		if !strings.Contains(out, want) {
			t.Errorf("the tooling container %q was not injected:\n%s", want, out)
		}
	}
	// 명세엔 없어야 한다(사용자가 지워서 데모를 망가뜨릴 여지를 두지 않는다).
	for _, n := range s.Nodes {
		if n.ID == "pqcota-ctl" || n.ID == "pqcota-demo-pg" {
			t.Errorf("a tooling container leaked into the spec nodes: %s", n.ID)
		}
	}
	// 전 세그먼트 참여 — 샘플은 corp·db 둘.
	if strings.Count(out, "networks: [corp, db]") < 2 {
		t.Errorf("ctl and pg must join every segment:\n%s", out)
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
			t.Errorf("compose is missing %q:\n%s", want, out)
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
		t.Errorf("pay-app should be a pqc edge target, java, with BC: %v", e)
	}
	if e := env("pay-db"); e["SSL_SERVER"] != "1" || e["SSL_APPS"] != "payment-gw,api-gw" {
		t.Errorf("pay-db should have several apps on a shared .so: %v", e)
	}
	if e := env("web-gw"); e["SSL_SERVER"] == "1" || e["PQC_SERVER"] == "1" {
		t.Errorf("web-gw is a client, so it should run no server: %v", e)
	}
}

// 트래픽 시나리오는 소스 노드에 붙는다(관측 구간을 그 노드에서 채운다).
func TestGroupsTraffic(t *testing.T) {
	out := GroupsINI(parse(t, sample))
	if !strings.Contains(out, `web-gw traffic="pqc:pay-app:8443 ssl:pay-db:4433"`) {
		t.Errorf("the source node got no edges:\n%s", out)
	}
	if !strings.Contains(out, `pay-db traffic=""`) {
		t.Errorf("a target-only node should have empty traffic:\n%s", out)
	}
}

// profiles.csv 헤더는 pqcota-profile(그리고 demo.sh)이 기대하는 것과 같아야 한다.
func TestProfilesHeader(t *testing.T) {
	out := ProfilesCSV(parse(t, sample))
	if !strings.HasPrefix(out, "node_id,display_name,environment,role,owner\n") {
		t.Errorf("header mismatch:\n%s", out)
	}
	if !strings.Contains(out, "web-gw,결제 웹,production,web,플랫폼팀") {
		t.Errorf("the profile row is missing:\n%s", out)
	}
	if !strings.Contains(out, "pay-app,pay-app,,,") { // name·profile 생략 시 id로 폴백
		t.Errorf("no fallback when the fields are omitted:\n%s", out)
	}
}

// manifest는 bash가 source해 그대로 쓴다(배열·연관배열 문법).
func TestManifest(t *testing.T) {
	out := ManifestEnv(parse(t, sample))
	for _, want := range []string{"NODES=(web-gw pay-app pay-db)", "EDGE_COUNT=2", `[web-gw]="결제 웹"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the manifest is missing %q:\n%s", want, out)
		}
	}
}

// ★ 정직성(§2.5) — 관측하지 못하는 종류·못 띄우는 fork는 조용히 넘어가지 않고 거부한다.
func TestValidateRejects(t *testing.T) {
	cases := map[string]string{
		"a kind we cannot observe":              "nodes:\n  - {id: a, kind: dotnet}\n",
		"a fork without s_server":               "nodes:\n  - {id: a, kind: openssl, openssl: {fork: boringssl}}\n",
		"unsupported version":                   "nodes:\n  - {id: a, kind: openssl, openssl: {fork: openssl, version: \"0.9\"}}\n",
		"invalid id":                            "nodes:\n  - {id: Web_GW, kind: java}\n",
		"duplicate id":                          "nodes:\n  - {id: a, kind: java}\n  - {id: a, kind: java}\n",
		"an edge to a node that does not exist": "nodes:\n  - {id: a, kind: java}\nedges:\n  - {from: a, to: ghost, proto: ssl}\n",
		"unsupported proto":                     "nodes:\n  - {id: a, kind: java}\nedges:\n  - {from: a, to: a, proto: quic}\n",
		"an undeclared network":                 "networks: [corp]\nnodes:\n  - {id: a, kind: java, networks: [nope]}\n",
		"empty nodes":                           "networks: [corp]\n",
	}
	for name, src := range cases {
		if err := Validate(parse(t, src)); err == nil {
			t.Errorf("%s: must be rejected", name)
		}
	}
	// 정상 명세는 통과.
	if err := Validate(parse(t, sample)); err != nil {
		t.Errorf("a valid spec was rejected: %v", err)
	}
}

// 따옴표 없는 version(3.0 → float)도 적힌 그대로 읽어야 한다 — 불친절한 파싱 오류 방지.
func TestUnquotedVersion(t *testing.T) {
	s := parse(t, "nodes:\n  - {id: a, kind: openssl, openssl: {fork: openssl, version: 3.0}}\n")
	if got := s.Nodes[0].version(); got != "3.0" {
		t.Errorf("an unquoted version read as %q — it must stay exactly as written", got)
	}
	if err := Validate(s); err != nil {
		t.Errorf("3.0 is a supported version: %v", err)
	}
}
