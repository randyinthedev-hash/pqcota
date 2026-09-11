package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
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
// 것**을 막는다. 곧 `ruleset`으로 시작하는 값과, 권위 있는 상수의 값을 그대로 베낀 값이다.
// 상수를 선언하는 파일 자신은 예외다.
//
// # 무엇을 보지 못하나
//
// 형식 문자열 안의 `ruleset`은 잡지 않는다(`"… · ruleset %s"`는 값이 아니라 이름표다).
// 변수에 담아 돌려 넘기는 것도 못 본다. 값이 상수에서 왔는지 끝까지 좇으려면 타입 해석이
// 필요하고, 그 해석은 이 검사기가 감당하는 범위 밖이다. 못 보는 것을 안 보는 척하지 않는다.

// rulesetExempt — 리터럴이 정의 그 자체인 파일. 권위 있는 상수를 선언하는 자리와, 무엇을
// 막을지 적어 둔 이 검사기 자신이다. 검사기를 빼지 않으면 **자기 규칙에 자기가 걸린다** —
// 실제로 그랬다(v0.7.4 CI). 추적 전 파일은 `git ls-files` 에 없어 손에서는 통과했다.
var rulesetExempt = map[string]bool{
	"pkg/discovery/normalize/pipeline.go": true,
	"tools/checkgates/ruleset.go":         true,
}

// rulesetConst — 그 상수의 값. 다른 데서 이 문자열을 베끼면 상수를 고쳐도 따라오지 않는다.
const rulesetConst = "pqcota-enrich/v1"

// rulesetPlaceholders — 규칙 판 식별자처럼 생긴 리터럴이 있는 자리.
func rulesetPlaceholders(files []string) ([]string, error) {
	fset := token.NewFileSet()
	var out []string
	for _, f := range files {
		if rulesetExempt[f] || strings.HasSuffix(f, "_test.go") {
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

func looksLikeRuleset(v string) bool {
	if v == rulesetConst {
		return true
	}
	l := strings.ToLower(v)
	return strings.HasPrefix(l, "ruleset-") || strings.HasPrefix(l, "ruleset_") || strings.HasPrefix(l, "ruleset/")
}
