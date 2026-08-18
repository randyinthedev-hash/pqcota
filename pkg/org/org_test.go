package org_test

import (
	"errors"
	"testing"

	"github.com/randyinthedev-hash/pqcota/pkg/discovery/history"
	"github.com/randyinthedev-hash/pqcota/pkg/inventory"
	"github.com/randyinthedev-hash/pqcota/pkg/org"
	"github.com/randyinthedev-hash/pqcota/pkg/provisioning"
)

// TestParseRejectsWhatCannotBeToldApart — 사람이 같게 읽고 기계가 다르게 읽는 이름을 받지 않는다.
func TestParseRejectsWhatCannotBeToldApart(t *testing.T) {
	for _, s := range []string{"", " ", "a", "Acme", "ACME", "-acme", "acme_corp", "acme.corp", "acme corp"} {
		if got, err := org.Parse(s); err == nil {
			t.Errorf("%q를 받아들였다 → %q", s, got)
		}
	}
	for _, s := range []string{"ac", "acme", "acme-corp", "org-2026", "a1"} {
		if _, err := org.Parse(s); err != nil {
			t.Errorf("%q를 거절했다: %v", s, err)
		}
	}
}

// TestEmptyIsNotAChoice — 빈 조직은 Default를 고른 것이 아니라 대다 만 것이다.
func TestEmptyIsNotAChoice(t *testing.T) {
	if _, err := org.Parse(""); !errors.Is(err, org.ErrEmpty) {
		t.Fatalf("빈 조직이 ErrEmpty가 아니다: %v", err)
	}
	if org.Default == "" {
		t.Fatal("Default가 빈 값이다 — \"조직 없음\"과 \"조직을 안 적었음\"이 같은 모양이 된다")
	}
}

// TestResolveFallsBackButNeverGuesses — 안 적은 것은 기본값으로, 틀리게 적은 것은 에러로.
func TestResolveFallsBackButNeverGuesses(t *testing.T) {
	got, err := org.Resolve("")
	if err != nil || got != org.Default {
		t.Fatalf("빈 입력이 Default로 풀리지 않는다: %q %v", got, err)
	}
	if _, err := org.Resolve("Acme"); err == nil {
		t.Fatal("오타를 기본값으로 삼켰다 — 그 행이 어디로 갔는지 알 수 없게 된다")
	}
}

// TestRequiredModeRefusesTheDefault — 여럿을 담는 배포에서 조직 없는 호출은 여는 자리에서 터진다.
func TestRequiredModeRefusesTheDefault(t *testing.T) {
	t.Setenv(org.RequireEnv, "1")
	if _, err := org.Resolve(""); !errors.Is(err, org.ErrDefaultNotAllowed) {
		t.Fatalf("필수 모드인데 기본 조직으로 열렸다: %v", err)
	}
	if got, err := org.Resolve("acme"); err != nil || got != org.ID("acme") {
		t.Fatalf("필수 모드에서 정상 조직이 막혔다: %q %v", got, err)
	}
}

// TestDefaultIsReservedInRequiredMode — `default`는 모양 규칙을 통과하므로 막지 않으면
// 고객 조직 ID로 배정될 수 있다. 배정되는 순간 단일 조직 시절 데이터와 한 조직이 된다.
func TestDefaultIsReservedInRequiredMode(t *testing.T) {
	if _, err := org.Parse(string(org.Default)); err != nil {
		t.Fatal("전제 확인: Default는 모양 규칙을 통과한다 — 그래서 예약이 필요하다")
	}
	t.Setenv(org.RequireEnv, "1")
	if _, err := org.Resolve(string(org.Default)); !errors.Is(err, org.ErrReserved) {
		t.Fatalf("필수 모드에서 예약 이름이 조직으로 열렸다: %v", err)
	}
}

// TestScopedIsSatisfiedByTheStores — 저장소 인터페이스를 안 건드리고 조직을 물을 수 있다.
func TestScopedIsSatisfiedByTheStores(t *testing.T) {
	var _ org.Scoped = (*history.MemStore)(nil)
	var _ org.Scoped = (*history.PgStore)(nil)
	var _ org.Scoped = (*inventory.MemMetaStore)(nil)
	var _ org.Scoped = (*inventory.PgMetaStore)(nil)
	var _ org.Scoped = (*provisioning.PgRecordStore)(nil)

	// history.Store로 받아 온 것도 타입 단언으로 물을 수 있다 — 인터페이스에 메서드를 더하지
	// 않았으므로 밖의 구현체는 깨지지 않는다.
	var st history.Store = history.NewMemStore()
	sc, ok := st.(org.Scoped)
	if !ok || sc.Org() != org.Default {
		t.Fatalf("Store를 org.Scoped로 물을 수 없다: ok=%v", ok)
	}
}
