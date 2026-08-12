// Package org — 인벤토리의 조직 축.
//
// 이 리포의 저장소는 한 조직만 담을 수도 있고 여럿을 담을 수도 있다. 어느 쪽이든 **모든 행은
// 조직에 속한다** — 속하지 않는 행을 만들 수 있게 두면 그 행은 나중에 누구 것인지 알 수 없다.
//
// 조직을 대지 않고 저장소를 열면 [Default]에 묶인다. 혼자 쓰는 사용자는 이 패키지를 모른 채로
// 지금까지처럼 쓰면 되고, 여럿을 담는 배포는 [Require] 모드로 그 기본값을 막는다.
package org

import (
	"errors"
	"fmt"
	"os"
	"regexp"
)

// ID — 조직 식별자. 저장소 핸들이 이 값에 묶이고, 모든 질의가 그것을 조건으로 건다.
type ID string

// Default — 조직을 대지 않고 연 저장소가 묶이는 곳.
//
// 빈 문자열이 아니다. 빈 값을 기본으로 두면 "조직 없음"과 "조직을 안 적었음"이 같은 모양이 되어,
// 나중에 그 행이 어느 쪽이었는지 갈라낼 수 없다. 이름이 있으면 적어도 **기본값에 들어갔다는
// 사실**이 남는다.
const Default ID = "default"

// RequireEnv — 이 환경변수가 "1"이면 [Default]로 저장소를 열 수 없다.
//
// 여러 조직을 한 저장소에 담는 배포에서 켠다. 조직을 대지 않은 호출이 조용히 기본값에 쓰는 대신
// 열리는 자리에서 터진다 — 데이터가 섞인 뒤에는 되돌릴 수 없기 때문이다.
const RequireEnv = "PQCOTA_REQUIRE_ORG"

var (
	// ErrEmpty — 빈 조직. 조직을 대려다 만 것이지 [Default]를 고른 것이 아니다.
	ErrEmpty = errors.New("조직이 비었다")
	// ErrShape — 모양이 규칙에 안 맞는다.
	ErrShape = errors.New("조직 이름의 모양이 규칙에 안 맞는다")
	// ErrDefaultNotAllowed — 필수 모드인데 조직 없이 열려 했다.
	ErrDefaultNotAllowed = fmt.Errorf("%s=1인데 조직을 대지 않았다 — 기본 조직으로 열 수 없다", RequireEnv)
)

// shape — 소문자·숫자·하이픈 2~64자, 하이픈으로 시작하지 않는다.
//
// 대소문자를 섞지 않는 이유: 섞을 수 있으면 `Acme`와 `acme`가 서로 다른 조직이 된다. 사람은
// 같은 것으로 읽고 기계는 다르게 읽는 자리를 만들지 않는다.
var shape = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

// Parse — 사용자가 적은 조직 이름을 검사해 ID로 만든다. 추측하지 않는다.
func Parse(s string) (ID, error) {
	if s == "" {
		return "", ErrEmpty
	}
	if !shape.MatchString(s) {
		return "", fmt.Errorf("%w: %q — 소문자·숫자·하이픈 2~64자여야 한다", ErrShape, s)
	}
	return ID(s), nil
}

// Resolve — 저장소 생성자가 쓰는 진입점. 빈 문자열은 [Default]로 풀되, 필수 모드에서는 거절한다.
//
// 조직을 **적었는데** 모양이 틀린 경우는 필수 모드와 무관하게 항상 에러다 — 오타를 기본값으로
// 삼키면 그 행이 어디로 갔는지 아무도 모른다.
func Resolve(s string) (ID, error) {
	if s == "" {
		if Required() {
			return "", ErrDefaultNotAllowed
		}
		return Default, nil
	}
	return Parse(s)
}

// Required — 필수 모드인가.
func Required() bool { return os.Getenv(RequireEnv) == "1" }
