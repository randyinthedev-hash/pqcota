// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

// 수집기·선언 도구의 서명(discovery·inventory)을 적재기의 노드 결합 검증(키 파일, inventory)에 이어 본다.
// 서명은 이미 노드를 덮으므로 수집기는 바뀌지 않는다 — 노드마다 다른 키를 주기만 하면 된다는 것을 실제 명령으로 잰다.

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	discoveryv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota-common/pkg/kernel/sign"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/resultio"
)

// 노드 둘(web-01·web-02)에 노드마다 다른 키, 선언 도구에 `*` 줄. netcap(web-01)·jvmscan(web-02)·두 선언 도구의
// 결과가 모두 적재된다. web-01의 키로 web-02를 적어 다시 서명한 결과는 거부되고, 본문(앱 선언 엣지)에 다른 노드를
// 적은 선언도 거부된다. 대조로 한 키를 두 노드에 적으면 같은 조작이 통과한다.
func TestCollectorsSignAndNodeBoundIngest(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pqcota-netcap is linux only")
	}
	netcap := buildCmd(t, "github.com/randyinthedev-hash/pqcota-discovery/cmd/pqcota-netcap")
	jvmscan := buildCmd(t, "github.com/randyinthedev-hash/pqcota-discovery/cmd/pqcota-jvmscan")
	declare := buildCmd(t, "github.com/randyinthedev-hash/pqcota-inventory/cmd/pqcota-declare")
	declAttr := buildCmd(t, "github.com/randyinthedev-hash/pqcota-inventory/cmd/pqcota-declare-attribution")
	ing := buildIngest(t)

	k1, k2, kc := newKey(t), newKey(t), newKey(t)
	dir := t.TempDir()
	out, errOut, code := runWith(t, []string{"PQCOTA_SIGN_KEY=" + k1.priv}, netcap, "web-01", "lo", "1")
	if code != 0 {
		t.Fatalf("netcap: exit %d\n%s", code, errOut)
	}
	writeFile(t, dir, "net.json", []byte(out))
	out, errOut, code = runWith(t, []string{"PQCOTA_SIGN_KEY=" + k2.priv, "JAVA_BIN=/nonexistent/java", "PQCOTA_JVM_AGENT="}, jvmscan, "web-02")
	if code != 0 {
		t.Fatalf("jvmscan: exit %d\n%s", code, errOut)
	}
	writeFile(t, dir, "jvm.json", []byte(out))
	csv := filepath.Join(t.TempDir(), "decl.csv")
	writeFile(t, filepath.Dir(csv), "decl.csv", []byte("web-01,openssl,libssl.so.3\n"))
	if _, errOut, code := runWith(t, []string{"PQCOTA_SIGN_KEY=" + kc.priv}, declare, "--out", dir, csv); code != 0 {
		t.Fatalf("declare: exit %d\n%s", code, errOut)
	}
	acsv := filepath.Join(t.TempDir(), "attr.csv")
	writeFile(t, filepath.Dir(acsv), "attr.csv", []byte("web-01,10.0.0.9:443,billing\nweb-02,10.0.0.10:443,payroll\n"))
	if _, errOut, code := runWith(t, []string{"PQCOTA_SIGN_KEY=" + kc.priv}, declAttr, "--out", dir, acsv); code != 0 {
		t.Fatalf("declare-attribution: exit %d\n%s", code, errOut)
	}
	results, flaws := resultio.LoadDir(dir)
	if len(flaws) > 0 || len(results) != 5 {
		t.Fatalf("results: %d, flaws %v", len(results), flaws)
	}

	keyFile := filepath.Join(t.TempDir(), "verify-keys")
	writeFile(t, filepath.Dir(keyFile), "verify-keys", []byte(fmt.Sprintf(
		"web-01 network-collector,jvm-collector %s\nweb-02 network-collector,jvm-collector %s\n* declaration-importer,attribution-importer %s\n",
		k1.pub, k2.pub, kc.pub)))
	env := []string{"PQCOTA_DSN=", "PQCOTA_REQUIRE_SIGNATURE=", "PQCOTA_VERIFY_KEY=", "PQCOTA_VERIFY_KEY_FILE=" + keyFile}
	out, errOut, code = runWith(t, env, ing, dir)
	if code != 0 || !strings.Contains(out, "bound to nodes (2 nodes, 3 keys, 1 entries for any node)") ||
		!strings.Contains(out, "accepted 3 · unregistered/no-anchor 0 · signature-rejected 0") {
		t.Fatalf("node-bound ingest of the four tools: exit %d\n%s%s", code, out, errOut)
	}

	// web-01의 netcap 결과를 web-02라고 바꿔(엣지의 출발 노드도 함께) web-01의 키로 다시 서명한다.
	forged := t.TempDir()
	var net *discoveryv1.CollectionResult
	for _, r := range results {
		if r.GetEnvelope().GetCollectorId() == "network-collector" {
			net = r
		}
	}
	net.Envelope.TargetNodeId = "web-02"
	for _, e := range net.GetObservedEdges() {
		e.SrcNodeId = "web-02"
	}
	if err := sign.SignAll(k1.priv, []*discoveryv1.CollectionResult{net}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, forged, "forged.json", encode(t, false, net))
	out, errOut, code = runWith(t, env, ing, forged)
	if code != 0 || !strings.Contains(out, "accepted 0 · unregistered/no-anchor 0 · signature-rejected 1") ||
		!strings.Contains(out, "does not match the keys registered for network-collector on web-02; it matches a key registered for jvm-collector on web-01, network-collector on web-01") {
		t.Fatalf("web-01's key naming web-02: exit %d\n%s%s", code, out, errOut)
	}

	// 본문의 노드: web-01 앱 선언의 엣지 출발 노드를 web-02로 바꿔 선언 키로 다시 서명한다.
	body := t.TempDir()
	for _, r := range results {
		if r.GetEnvelope().GetCollectorId() == "attribution-importer" && r.GetEnvelope().GetTargetNodeId() == "web-01" {
			r.ObservedEdges[0].SrcNodeId = "web-02"
			if err := sign.SignAll(kc.priv, []*discoveryv1.CollectionResult{r}); err != nil {
				t.Fatal(err)
			}
			writeFile(t, body, "body.json", encode(t, false, r))
		}
	}
	out, errOut, code = runWith(t, env, ing, body)
	if code != 0 || !strings.Contains(out, `edge source node "web-02" is outside the node this result was signed for ("web-01")`) ||
		!strings.Contains(out, "accepted 0 · unregistered/no-anchor 0 · signature-rejected 1") {
		t.Fatalf("a declaration naming another node in its body: exit %d\n%s%s", code, out, errOut)
	}

	// 대조: 같은 키를 두 노드에 적으면 노드를 바꾼 조작이 통과한다(공유 키는 노드를 구별하지 않는다).
	shared := filepath.Join(t.TempDir(), "shared")
	writeFile(t, filepath.Dir(shared), "shared", []byte(fmt.Sprintf("web-01 network-collector %s\nweb-02 network-collector %s\n", k1.pub, k1.pub)))
	out, errOut, code = runWith(t, []string{"PQCOTA_DSN=", "PQCOTA_VERIFY_KEY=", "PQCOTA_VERIFY_KEY_FILE=" + shared}, ing, forged)
	if code != 0 || !strings.Contains(out, "accepted 1 · unregistered/no-anchor 0 · signature-rejected 0") ||
		!strings.Contains(errOut, "is listed for 2 nodes (web-01, web-02)") {
		t.Fatalf("shared key control: exit %d\n%s%s", code, out, errOut)
	}
}
