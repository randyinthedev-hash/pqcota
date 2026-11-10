// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

// jvm collector의 출력(discovery)을 적재기의 읽기와 관문(inventory)에 이어 본다.
//
// v0.10.6까지 jvmscan의 「한 줄에 하나」 출력이 결과마다 여러 줄이었다(protojson의 Indent). JVM이 하나면
// 파일 전체가 객체 하나로 읽혀 통했고, 둘 이상이면 줄마다 읽다가 모두 실패해 적재 요청 전체가
// 거부됐다. 수집기와 적재기가 서로 다른 리포라 한쪽 시험만으로는 이 어긋남이 보이지 않았다.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	discoveryv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota-discovery/collectors/jvm"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/resultio"
)

const providers = `PQCOTA_PROVIDERS_BEGIN
0|SUN|21|sun.security.provider.Sun
1|SunJCE|21|com.sun.crypto.provider.SunJCE
PQCOTA_PROVIDERS_END`

// encode — 수집기가 내는 그대로 바이트로 만든다.
func encode(t *testing.T, oneLine bool, rs ...*discoveryv1.CollectionResult) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jvm.EncodeResults(&buf, rs, oneLine); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jvmResult(node, ident string) *discoveryv1.CollectionResult {
	return jvm.BuildResultFor(node, jvm.ParseProviders(providers), ident)
}

// 결과 둘 이상을 한 줄 모드로 내면 적재기의 디코더가 모두 읽는다. 단일 객체 출력도 계속 읽힌다.
func TestJVMOutputDecodesInTheIngester(t *testing.T) {
	many := encode(t, true, jvmResult("pay-app", "/opt/a/app.jar"), jvmResult("pay-app", "/opt/b/app.jar"), jvmResult("pay-app", "/opt/CryptoApp.java"))
	got, flaws := resultio.Decode(many)
	if len(flaws) > 0 || len(got) != 3 {
		t.Fatalf("결과 셋의 한 줄 출력을 읽지 못했다: 읽음 %d · 못 읽음 %v", len(got), flaws)
	}
	single := encode(t, false, jvmResult("pay-app", "/opt/CryptoApp.java"))
	if got, flaws := resultio.Decode(single); len(flaws) > 0 || len(got) != 1 {
		t.Fatalf("단일 객체 출력을 읽지 못했다: 읽음 %d · 못 읽음 %v", len(got), flaws)
	}
}

// buildIngest — 이 작업 공간의 적재기를 빌드한다(형제 리포를 replace로 읽는다).
func buildIngest(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "pqcota-ingest")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/randyinthedev-hash/pqcota-inventory/cmd/pqcota-ingest")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build pqcota-ingest: %v\n%s", err, out)
	}
	return bin
}

func ingest(t *testing.T, bin, dir string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, dir)
	cmd.Env = append(os.Environ(), "PQCOTA_DSN=", "PQCOTA_VERIFY_KEY=", "PQCOTA_REQUIRE_SIGNATURE=")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func writeFile(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// JVM이 여럿인 노드의 결과를 다른 노드의 결과와 한 적재 요청에 넣어도 형식 오류 없이 적재된다.
// 손상된 줄이 하나라도 있으면 그 적재 요청 전체가 여전히 거부된다(일부만 적재하지 않는다).
func TestIngestGateWithManyJVMs(t *testing.T) {
	bin := buildIngest(t)

	ok := t.TempDir()
	writeFile(t, ok, "pay-app-jca.jsonl", encode(t, true, jvmResult("pay-app", "/opt/a/app.jar"), jvmResult("pay-app", "/opt/b/app.jar")))
	writeFile(t, ok, "web-gw-jca.json", encode(t, false, jvmResult("web-gw", "/srv/gw.jar")))
	out, code := ingest(t, bin, ok)
	if code != 0 || !strings.Contains(out, "ingest result: accepted 3 ") || strings.Contains(out, "could not decode") {
		t.Fatalf("JVM 둘인 노드와 다른 노드를 함께 적재하지 못했다(종료 %d):\n%s", code, out)
	}

	bad := t.TempDir()
	lines := encode(t, true, jvmResult("pay-app", "/opt/a/app.jar"), jvmResult("pay-app", "/opt/b/app.jar"))
	writeFile(t, bad, "pay-app-jca.jsonl", append(lines, []byte("{\"envelope\":\n")...))
	writeFile(t, bad, "web-gw-jca.json", encode(t, false, jvmResult("web-gw", "/srv/gw.jar")))
	out, code = ingest(t, bin, bad)
	if code == 0 || !strings.Contains(out, "refusing to ingest a partial set") || strings.Contains(out, "ingest result:") {
		t.Fatalf("손상된 줄이 있는데 적재 요청 전체가 거부되지 않았다(종료 %d):\n%s", code, out)
	}
}
