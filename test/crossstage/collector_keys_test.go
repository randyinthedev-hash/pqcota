// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

// 수집기·선언 도구의 서명(discovery·inventory)을 적재기의 ID 결합 검증(inventory)에 이어 본다. 서명하는 쪽과
// 검증하는 쪽이 서로 다른 명령이라, 한쪽 시험만으로는 「서명하지 않는 수집기」가 보이지 않았다(v0.14.0까지
// netcap·jvmscan·두 선언 도구는 서명하지 않았다).

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	discoveryv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota-common/pkg/kernel/sign"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/resultio"
)

func buildCmd(t *testing.T, pkg string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), filepath.Base(pkg))
	if out, err := exec.Command("go", "build", "-o", bin, pkg).CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, out)
	}
	return bin
}

func runWith(t *testing.T, env []string, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return o.String(), e.String(), code
}

type keyPair struct{ pub, priv string }

func newKey(t *testing.T) keyPair {
	t.Helper()
	pub, priv, err := sign.Generate()
	if err != nil {
		t.Fatal(err)
	}
	return keyPair{pub, priv}
}

// 네 도구(netcap, jvmscan, declare, declare-attribution)가 PQCOTA_SIGN_KEY로 서명하고, ID마다 다른 키를 등록한
// 결합 모드에서 모두 적재된다. jvmscan 결과의 collector_id를 network-collector로 바꿔 자기 키로 다시 서명하면
// 거부된다. 같은 키를 두 ID에 등록하면 같은 조작이 통과한다(공유 키는 ID를 구별하지 않는다).
func TestCollectorsSignAndBoundIngestAcceptsOnlyTheirOwnID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pqcota-netcap is linux only")
	}
	netcap := buildCmd(t, "github.com/randyinthedev-hash/pqcota-discovery/cmd/pqcota-netcap")
	jvmscan := buildCmd(t, "github.com/randyinthedev-hash/pqcota-discovery/cmd/pqcota-jvmscan")
	declare := buildCmd(t, "github.com/randyinthedev-hash/pqcota-inventory/cmd/pqcota-declare")
	declAttr := buildCmd(t, "github.com/randyinthedev-hash/pqcota-inventory/cmd/pqcota-declare-attribution")
	ing := buildIngest(t)

	kNet, kJvm, kDecl, kAttr := newKey(t), newKey(t), newKey(t), newKey(t)
	dir := t.TempDir()

	// netcap: 1초 관측. CAP_NET_RAW가 없으면 갭만 담은 결과를 낸다. 어느 쪽이든 서명한다.
	out, errOut, code := runWith(t, []string{"PQCOTA_SIGN_KEY=" + kNet.priv}, netcap, "web-01", "lo", "1")
	if code != 0 {
		t.Fatalf("netcap: exit %d\n%s", code, errOut)
	}
	writeFile(t, dir, "net.json", []byte(out))
	// jvmscan: java가 없으면 프로브가 실패한 결과(갭)를 낸다. 그것도 서명한다.
	out, errOut, code = runWith(t, []string{"PQCOTA_SIGN_KEY=" + kJvm.priv, "JAVA_BIN=/nonexistent/java", "PQCOTA_JVM_AGENT="}, jvmscan, "web-01")
	if code != 0 {
		t.Fatalf("jvmscan: exit %d\n%s", code, errOut)
	}
	writeFile(t, dir, "jvm.json", []byte(out))
	csv := filepath.Join(t.TempDir(), "decl.csv")
	writeFile(t, filepath.Dir(csv), "decl.csv", []byte("web-01,openssl,libssl.so.3\n"))
	if _, errOut, code := runWith(t, []string{"PQCOTA_SIGN_KEY=" + kDecl.priv}, declare, "--out", dir, csv); code != 0 {
		t.Fatalf("declare: exit %d\n%s", code, errOut)
	}
	acsv := filepath.Join(t.TempDir(), "attr.csv")
	writeFile(t, filepath.Dir(acsv), "attr.csv", []byte("web-01,10.0.0.9:443,billing\n"))
	if _, errOut, code := runWith(t, []string{"PQCOTA_SIGN_KEY=" + kAttr.priv}, declAttr, "--out", dir, acsv); code != 0 {
		t.Fatalf("declare-attribution: exit %d\n%s", code, errOut)
	}

	results, flaws := resultio.LoadDir(dir)
	if len(flaws) > 0 || len(results) != 4 {
		t.Fatalf("results: %d, flaws %v", len(results), flaws)
	}
	for _, r := range results {
		if r.GetEnvelope().GetSignature() == "" {
			t.Errorf("%s did not sign", r.GetEnvelope().GetCollectorId())
		}
	}

	bound := "network-collector:" + kNet.pub + ",jvm-collector:" + kJvm.pub +
		",declaration-importer:" + kDecl.pub + ",attribution-importer:" + kAttr.pub
	env := []string{"PQCOTA_DSN=", "PQCOTA_REQUIRE_SIGNATURE=", "PQCOTA_VERIFY_KEY=" + bound}
	out, errOut, code = runWith(t, env, ing, dir)
	if code != 0 || !strings.Contains(out, "bound to collector IDs (4 IDs, 4 keys)") ||
		!strings.Contains(out, "accepted 3 · unregistered/no-anchor 0 · signature-rejected 0") {
		t.Fatalf("bound ingest of the four tools: exit %d\n%s%s", code, out, errOut)
	}

	// jvmscan 결과를 network-collector라고 바꿔 jvm 키로 다시 서명한다.
	forged := t.TempDir()
	for _, r := range results {
		if r.GetEnvelope().GetCollectorId() != "jvm-collector" {
			continue
		}
		r.Envelope.CollectorId = "network-collector"
		if err := sign.SignAll(kJvm.priv, []*discoveryv1.CollectionResult{r}); err != nil {
			t.Fatal(err)
		}
		writeFile(t, forged, "forged.json", encode(t, false, r))
	}
	out, errOut, code = runWith(t, env, ing, forged)
	if code != 0 || !strings.Contains(out, "accepted 0 · unregistered/no-anchor 0 · signature-rejected 1") ||
		!strings.Contains(out, "does not match the keys registered for network-collector; it matches a key registered for jvm-collector") {
		t.Fatalf("a jvm result relabelled as network-collector: exit %d\n%s%s", code, out, errOut)
	}

	// 대조: 같은 키를 두 ID에 등록하면 같은 조작이 통과한다.
	shared := []string{"PQCOTA_DSN=", "PQCOTA_VERIFY_KEY=network-collector:" + kJvm.pub + ",jvm-collector:" + kJvm.pub}
	out, errOut, code = runWith(t, shared, ing, forged)
	if code != 0 || !strings.Contains(out, "accepted 1 · unregistered/no-anchor 0 · signature-rejected 0") {
		t.Fatalf("shared key control: exit %d\n%s%s", code, out, errOut)
	}
}
