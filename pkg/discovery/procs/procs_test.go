package procs_test

import (
	"os"
	"path/filepath"
	"testing"

	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/procs"
)

func TestMatch(t *testing.T) {
	exe, cmd, cg := "/opt/java/bin/java", "java -jar /opt/payment/app.jar", "0::/system.slice/payment.service"

	// systemd_unit — cgroup 포함
	if !procs.Match(exe, cmd, cg, &discoveryv1.ProcessMatch{SystemdUnit: "payment.service"}) {
		t.Error("systemd_unit 매칭 실패")
	}
	if procs.Match(exe, cmd, "0::/system.slice/other.service", &discoveryv1.ProcessMatch{SystemdUnit: "payment.service"}) {
		t.Error("다른 유닛이 매칭됨")
	}
	// exe_path 정확
	if !procs.Match(exe, cmd, cg, &discoveryv1.ProcessMatch{ExePath: "/opt/java/bin/java"}) {
		t.Error("exe_path 매칭 실패")
	}
	// cmdline_regex
	if !procs.Match(exe, cmd, cg, &discoveryv1.ProcessMatch{CmdlineRegex: "payment/app\\.jar"}) {
		t.Error("cmdline_regex 매칭 실패")
	}
	// AND: 둘 다 만족해야
	if procs.Match(exe, cmd, cg, &discoveryv1.ProcessMatch{ExePath: "/opt/java/bin/java", SystemdUnit: "other.service"}) {
		t.Error("AND 위반(한 조건 불만족인데 매칭)")
	}
	// 규칙 없음 → false(전체 매칭 방지)
	if procs.Match(exe, cmd, cg, &discoveryv1.ProcessMatch{}) || procs.Match(exe, cmd, cg, nil) {
		t.Error("빈 규칙이 매칭됨")
	}
}

// Resolve — 가짜 /proc 트리로 exe/cmdline 기반 해소 검증.
func TestResolve(t *testing.T) {
	root := t.TempDir()
	mkProc := func(pid, cmdline, target string) {
		d := filepath.Join(root, pid)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "cmdline"), []byte(cmdline), 0o644)
		os.WriteFile(filepath.Join(d, "cgroup"), []byte("0::/system.slice/x"), 0o644)
		os.Symlink(target, filepath.Join(d, "exe"))
	}
	mkProc("101", "java\x00-jar\x00/opt/app.jar", "/opt/java/bin/java")
	mkProc("202", "nginx", "/usr/sbin/nginx")
	os.WriteFile(filepath.Join(root, "cpuinfo"), []byte("x"), 0o644) // 숫자 아닌 항목

	got, err := procs.Resolve(root, &discoveryv1.ProcessMatch{ExePath: "/opt/java/bin/java"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].GetPid() != 101 {
		t.Fatalf("해소 결과: %+v (want pid 101 하나)", got)
	}
	if got[0].GetCmdline() != "java -jar /opt/app.jar" {
		t.Errorf("cmdline 정규화 오류: %q", got[0].GetCmdline())
	}
}
