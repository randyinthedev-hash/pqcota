package main

// 검사기 자신을 고정한다. 「실제 호출을 임시로 지워 본다」는 도입 검증이지 회귀 방지가 아니다.
// 나중에 이 파일을 손볼 때 별칭 import·동명 함수·테스트 제외 가운데 하나가 깨져도 아무도 모른다.
//
// 가장 위험한 것은 **거짓 통과**다. `os.Executable`을 배선으로 세면 없는 보장을 있다고 말하게 되고,
// 그때 게이트는 초록인 채로 아무것도 지키지 않는다.

import (
	"strings"
	"testing"
)

const rule = "testdata/rule/rule.go" // GATE: 배선 필수가 붙은 provisioning.Executable

func run(t *testing.T, files ...string) (miss, notes []string) {
	t.Helper()
	miss, notes, err := check(files)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return miss, notes
}

func TestCalledFromProductCode(t *testing.T) {
	miss, _ := run(t, rule, "testdata/wired/main.go")
	if len(miss) != 0 {
		t.Errorf("정상 호출을 못 봤다: %v", miss)
	}
}

// 별칭을 놓치면 배선된 것을 막는다 — 거짓 실패다.
func TestAliasImportCounts(t *testing.T) {
	miss, _ := run(t, rule, "testdata/alias/main.go")
	if len(miss) != 0 {
		t.Errorf("별칭 import를 놓쳤다: %v", miss)
	}
}

// 동명 함수를 세면 없는 배선을 있다고 한다 — 거짓 통과이고, 더 위험하다.
func TestSameNameInAnotherPackageDoesNotCount(t *testing.T) {
	miss, _ := run(t, rule, "testdata/samename/main.go")
	if len(miss) != 1 {
		t.Fatalf("os.Executable을 배선으로 셌다: %v", miss)
	}
	if !strings.Contains(miss[0], "Executable") {
		t.Errorf("무엇이 빠졌는지 말하지 않는다: %s", miss[0])
	}
}

// 테스트만 부르는 것은 배선이 아니다 — 이 검사기가 존재하는 이유다.
func TestTestOnlyCallIsNotWiring(t *testing.T) {
	miss, _ := run(t, rule, "testdata/testonly/main_test.go")
	if len(miss) != 1 {
		t.Errorf("테스트 호출을 배선으로 셌다: %v", miss)
	}
}

// 보류는 통과시키되 조용하지 않다.
func TestPendingPassesButIsAnnounced(t *testing.T) {
	miss, notes := run(t, "testdata/pending/rule.go")
	if len(miss) != 0 {
		t.Errorf("보류를 막았다: %v", miss)
	}
	if len(notes) != 1 {
		t.Fatalf("보류를 고지하지 않았다: %v", notes)
	}
	if !strings.Contains(notes[0], "VerifyFrom") || !strings.Contains(notes[0], "§8") {
		t.Errorf("무엇을 왜 미뤘는지 말하지 않는다: %s", notes[0])
	}
}

// 리시버가 붙은 것은 등록하지 않는다 — 좌변이 변수라 패키지로 가릴 수 없기 때문이다.
func TestMethodsAreNotRegistered(t *testing.T) {
	miss, notes := run(t, "testdata/method/rule.go")
	if len(miss) != 0 || len(notes) != 0 {
		t.Errorf("메서드를 등록했다: miss=%v notes=%v", miss, notes)
	}
}
