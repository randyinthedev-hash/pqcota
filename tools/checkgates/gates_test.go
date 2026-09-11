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

// 규칙 판을 자기 문자열로 찍으면 막는다. 아무것도 실패하지 않고 이력 비교만 조용히
// 무의미해지는 자리라, 사람 눈으로는 릴리스 두 번을 지나도 안 보였다.
func TestRulesetPlaceholderIsBlocked(t *testing.T) {
	hits, err := rulesetPlaceholders([]string{"testdata/ruleset/main.go"})
	if err != nil {
		t.Fatalf("rulesetPlaceholders: %v", err)
	}
	if len(hits) != 4 {
		t.Fatalf("자리표시자와 계열 셋을 잡아야 한다: %v", hits)
	}
	for _, want := range []string{"ruleset-demo", "pqcota-enrich/v1", "pqcota-enrich/v0", "pqcota-enrich/v9"} {
		found := false
		for _, h := range hits {
			if strings.Contains(h, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q를 놓쳤다: %v", want, hits)
		}
	}
	for _, h := range hits {
		if !strings.Contains(h, "normalize.RulesetVersion") {
			t.Errorf("무엇을 써야 하는지 말하지 않는다: %s", h)
		}
	}
}

// 형식 문자열의 `ruleset`은 이름표다. 잡으면 거짓 실패가 나고, 그런 게이트는 곧 꺼진다.
func TestRulesetLabelIsNotAValue(t *testing.T) {
	hits, err := rulesetPlaceholders([]string{"testdata/ruleset/main.go"})
	if err != nil {
		t.Fatalf("rulesetPlaceholders: %v", err)
	}
	for _, h := range hits {
		if strings.Contains(h, "snapshot %s") {
			t.Errorf("이름표를 값으로 셌다: %s", h)
		}
	}
}

// ★ 검사기는 자기 검사를 지나야 한다.
//
// 처음에는 막으려는 값을 여기에 베껴 두고 이 파일을 예외로 뺐다. 그것이 **막으려는 바로 그
// 복제**다: 값을 베끼면 상수를 고쳐도 따라오지 않아, 판이 올라간 다음부터는 새 값을 베낀
// 자리를 못 잡는다. 예외로 빼 두면 그 사실이 영원히 드러나지 않는다.
func TestTheCheckerPassesItsOwnRule(t *testing.T) {
	hits, err := rulesetPlaceholders([]string{"ruleset.go"})
	if err != nil {
		t.Fatalf("rulesetPlaceholders: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("검사기가 자기 규칙에 걸린다 — 값을 베껴 두었다는 뜻이다: %v", hits)
	}
}

// ★ 지금 판만 잡으면 판이 오른 다음이 빈다.
//
// 상수가 `v2` 로 올라가면 어딘가 남은 `v1` 복사본은 「지금 값과 다르다」로 지나간다. 그것이
// 바로 잡아야 하는 것이다: 베낀 값은 상수를 고쳐도 따라오지 않으므로, 옛 판을 찍는 코드가
// 조용히 남는다. **이 검사는 지금 상수의 값에 기대지 않는다** — 계열의 다른 판을 넣어 본다.
func TestPastAndFutureRulesetsAreCaughtToo(t *testing.T) {
	for _, v := range []string{"pqcota-enrich/v0", "pqcota-enrich/v9", "pqcota-enrich/"} {
		if !looksLikeRuleset(v) {
			t.Errorf("계열의 다른 판 %q를 놓쳤다 — 판이 오른 뒤 옛 복사본이 남는다", v)
		}
	}
	if fam := rulesetFamily(); fam == "" || strings.HasSuffix(fam, "/") == false {
		t.Errorf("계열을 상수에서 못 뽑았다: %q", fam)
	}
	// 계열 밖은 그대로 둔다. 거짓 실패가 나는 게이트는 곧 꺼진다.
	for _, v := range []string{"pqcaton-plan/v1", "pqcota-enrich", "enrich/v1"} {
		if looksLikeRuleset(v) {
			t.Errorf("계열 밖의 %q를 걸었다", v)
		}
	}
}

// 이름만으로는 값이 아니다. 마디가 붙어야 식별자다.
func TestBareWordIsNotARulesetID(t *testing.T) {
	for _, v := range []string{"ruleset", "rulesetting", "-_/"} {
		if looksLikeRuleset(v) {
			t.Errorf("%q를 규칙 판 식별자로 셌다", v)
		}
	}
	for _, v := range []string{"ruleset-demo", "ruleset_1", "ruleset/v2"} {
		if !looksLikeRuleset(v) {
			t.Errorf("%q를 놓쳤다", v)
		}
	}
}
