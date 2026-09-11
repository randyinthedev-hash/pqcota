package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/randyinthedev-hash/pqcota/pkg/discovery/normalize"
)

// 규칙 판 자리표시자 검사 — **상수는 있는데 제품이 자기 문자열을 쓰는** 자리를 막는다.
//
// 배선 검사와 같은 부류다. 파생값이 어느 규칙에서 나왔는지는 `normalize.RulesetVersion`
// 하나가 말하기로 정해 두었는데, 적재 명령이 자기 문자열을 넘기면 그 약속이 깨진다.
// 깨져도 아무것도 실패하지 않는 것이 문제다: 스냅샷은 그대로 저장되고, 이력 비교만 조용히
// 무의미해진다. 모든 스냅샷이 같은 자리표시자를 달면 「규칙이 달라 파생값이 움직인 것인지
// 실제 변화인지」를 가르는 근거가 통째로 사라진다.
//
// 실제로 그랬다: `pqcota-ingest`와 `pqcota-cbom-ingest`가 실제 적재에 `"ruleset-demo"`를
// 찍고 있었다. 둘 다 문서상 정식 종단이다.
//
// # 무엇을 보나
//
// 추적 중인 Go 파일(테스트·`gen/` 제외)의 문자열 리터럴 가운데, **규칙 판 식별자처럼 생긴
// 것**을 막는다. 곧 `ruleset`으로 시작하는 값과, 권위 있는 상수와 **같은 계열**인 값이다.
// 계열로 보는 것은 판이 오른 뒤에 남은 옛 판 복사본까지 잡기 위해서다(`rulesetFamily`).
// 상수를 선언하는 파일 자신은 예외다.
//
// # 무엇을 보지 못하나
//
// 형식 문자열 안의 `ruleset`은 잡지 않는다(`"… · ruleset %s"`는 값이 아니라 이름표다).
// 변수에 담아 돌려 넘기는 것도 못 본다. 값이 상수에서 왔는지 끝까지 좇으려면 타입 해석이
// 필요하고, 그 해석은 이 검사기가 감당하는 범위 밖이다. 못 보는 것을 안 보는 척하지 않는다.

// rulesetHome — 리터럴이 정의 그 자체인 파일. 권위 있는 상수를 선언하는 자리 하나다.
//
// **이 검사기 자신은 예외로 두지 않는다.** 처음에는 상수의 값을 여기에 베껴 적어 두고 그
// 때문에 이 파일을 빼야 했는데, 그것이 **막으려는 바로 그 복제**였다. 값을 베끼면 상수를
// 고쳐도 따라오지 않으므로, 판이 올라간 다음부터는 새 값을 베낀 자리를 못 잡는다. 그래서
// 상수는 `normalize` 에서 직접 읽고, 이름은 마디로만 본다(아래 [looksLikeRuleset]).
// 자기 자신을 검사 대상에서 빼는 검사기는 그 규칙이 무엇을 막는지 보장하지 못한다.
const rulesetHome = "pkg/discovery/normalize/pipeline.go"

// rulesetPlaceholders — 규칙 판 식별자처럼 생긴 리터럴이 있는 자리.
func rulesetPlaceholders(files []string) ([]string, error) {
	fset := token.NewFileSet()
	var out []string
	for _, f := range files {
		if f == rulesetHome || strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil || !looksLikeRuleset(v) {
				return true
			}
			out = append(out, fmt.Sprintf("%s:%d: %q — `normalize.RulesetVersion`을 쓸 것",
				f, fset.Position(lit.Pos()).Line, v))
			return true
		})
	}
	return out, nil
}

// rulesetFamily — 권위 있는 값의 **계열**. 마지막 마디 앞까지다(`pqcota-enrich/v1` 이면
// `pqcota-enrich/`).
//
// **지금 값 하나만 보면 판이 오른 뒤가 빈다.** 상수가 `v2` 로 올라가면 어딘가 남은
// `pqcota-enrich/v1` 복사본은 「지금 값과 다르니 자리표시자가 아니다」로 지나간다. 그것이
// 바로 잡아야 하는 것이다: 베낀 값은 상수를 고쳐도 따라오지 않아 **옛 판을 찍는 코드가
// 조용히 남는다.** 계열로 보면 과거·미래 판이 함께 걸린다.
//
// 마디가 없는 값이면 빈 문자열을 돌려준다. 그때 계열은 「전부」가 되어 아무 문자열이나
// 걸리기 때문이다.
func rulesetFamily() string {
	i := strings.LastIndex(normalize.RulesetVersion, "/")
	if i < 0 {
		return ""
	}
	return normalize.RulesetVersion[:i+1]
}

// looksLikeRuleset — 규칙 판 식별자처럼 생긴 값인가.
//
// 권위 있는 값의 계열에 들거나, `ruleset` 뒤에 마디 기호가 붙은 것이다. 이름과 기호를
// **따로** 두는 것은 이 함수가 자기 리터럴에 걸리지 않게 하기 위해서다. `"ruleset"` 만으로는
// 마디가 없어 걸리지 않고, 기호 목록에는 이름이 없다. 계열은 상수에서 계산하므로 여기에
// 그 값이 적히지 않는다.
func looksLikeRuleset(v string) bool {
	if v == normalize.RulesetVersion {
		return true
	}
	if fam := rulesetFamily(); fam != "" && strings.HasPrefix(v, fam) {
		return true
	}
	const name = "ruleset"
	l := strings.ToLower(v)
	if !strings.HasPrefix(l, name) || len(l) == len(name) {
		return false
	}
	return strings.ContainsRune("-_/", rune(l[len(name)]))
}
