package scope_test

import (
	"strings"
	"testing"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/kernel/scope"
)

func openssl(lib string, apps ...string) *discoveryv1.Finding {
	return &discoveryv1.Finding{
		CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		RuntimeAxes:   &discoveryv1.Finding_Openssl{Openssl: &discoveryv1.OpensslAxes{Lib: lib}},
		AppKeys:       apps,
	}
}

// 정책이 없으면 관측된 것 전부가 관리 대상이다(기본 포함 — 스코프를 안 쓰는 사용자를 막지 않는다).
func TestNoPolicyKeepsEverything(t *testing.T) {
	var p *scope.AssetPolicy
	kept, excluded := p.Apply([]*discoveryv1.Finding{openssl("libssl.so.3"), openssl("libcrypto.so.3")})
	if len(kept) != 2 || excluded != 0 {
		t.Errorf("정책 없으면 전부 유지여야 함: kept=%d excluded=%d", len(kept), excluded)
	}
}

// 잡음(패키지 런타임 등)을 앱 이름으로 걸러낸다 — 이게 없으면 인벤토리가 못 쓰게 된다.
func TestExcludeByAppKeyGlob(t *testing.T) {
	p, err := scope.LoadAssetPolicy(strings.NewReader(`action,runtime,lib,app_key,note
exclude,*,*,/usr/bin/python*,python 런타임 — 관리 대상 아님
`))
	if err != nil {
		t.Fatal(err)
	}
	kept, excluded := p.Apply([]*discoveryv1.Finding{
		openssl("libcrypto.so.3", "/usr/bin/python3.12"),
		openssl("libssl.so.3", "/opt/apps/payment-gw"),
	})
	if len(kept) != 1 || excluded != 1 {
		t.Fatalf("kept=%d excluded=%d, want 1/1", len(kept), excluded)
	}
	if kept[0].GetOpenssl().GetLib() != "libssl.so.3" {
		t.Errorf("남은 자산이 틀림: %s", kept[0].GetOpenssl().GetLib())
	}
}

// include가 exclude를 이긴다 — "이 계열은 전부 빼되 이것만 예외"를 쓸 수 있어야 한다.
func TestIncludeOverridesExclude(t *testing.T) {
	p, _ := scope.LoadAssetPolicy(strings.NewReader(`
exclude,openssl,libcrypto.so.*,*,전부 제외
include,openssl,libcrypto.so.3,/opt/apps/payment-gw,결제 게이트웨이만 예외
`))
	kept, excluded := p.Apply([]*discoveryv1.Finding{
		openssl("libcrypto.so.3", "/opt/apps/payment-gw"), // 예외 → 유지
		openssl("libcrypto.so.1", "/usr/sbin/sshd"),       // 제외
	})
	if len(kept) != 1 || excluded != 1 {
		t.Fatalf("kept=%d excluded=%d, want 1/1", len(kept), excluded)
	}
	if kept[0].GetOpenssl().GetLib() != "libcrypto.so.3" {
		t.Errorf("include 예외가 안 먹음: %s", kept[0].GetOpenssl().GetLib())
	}
}

// 공유 .so는 앱이 여럿이다 — 하나만 매치해도 규칙이 걸린다.
func TestMultiAppAttribution(t *testing.T) {
	p, _ := scope.LoadAssetPolicy(strings.NewReader("exclude,*,*,/usr/sbin/sshd,\n"))
	kept, excluded := p.Apply([]*discoveryv1.Finding{
		openssl("libcrypto.so.3", "/opt/apps/payment-gw", "/usr/sbin/sshd"),
	})
	if len(kept) != 0 || excluded != 1 {
		t.Errorf("쓰는 앱 중 하나만 맞아도 매치해야 함: kept=%d excluded=%d", len(kept), excluded)
	}
}

// ★ 리뷰 지적 — 공유 .so의 영향 반경(blast radius): 테스트 앱 하나를 빼려다 그 .so를 함께
// 쓰는 운영 앱까지 빠진다. 규칙은 순서대로·뒤가 이기므로, 운영 앱을 되살리는 include를 exclude
// 뒤에 두어 구제한다. include는 "무조건 우선"이 아니라 순서 기반임을 함께 못박는다(TV-SCOPE-3×TV-SCOPE-4).
func TestSharedLibExcludeRescuedByTrailingInclude(t *testing.T) {
	// libcrypto.so.3을 테스트 앱과 운영 앱이 함께 로드(여러 앱에 걸침).
	shared := func() *discoveryv1.Finding {
		return openssl("libcrypto.so.3", "/opt/apps/internal-test-x", "/opt/apps/payment-gw")
	}

	// (1) 구제 없음: 테스트 앱만 겨냥해 빼도 공유 .so 전체가 빠진다(운영 payment-gw 몫까지).
	p1, _ := scope.LoadAssetPolicy(strings.NewReader(
		"exclude,openssl,libcrypto.so.*,/opt/apps/internal-test-*,테스트 앱 잡음\n"))
	if kept, excl := p1.Apply([]*discoveryv1.Finding{shared()}); len(kept) != 0 || excl != 1 {
		t.Fatalf("공유 .so는 쓰는 앱 하나만 맞아도 전체가 빠짐(blast radius): kept=%d excl=%d", len(kept), excl)
	}

	// (2) 구제: 운영 앱을 되살리는 include를 exclude '뒤에' 두면 보존된다(뒤가 이긴다).
	p2, _ := scope.LoadAssetPolicy(strings.NewReader(
		"exclude,openssl,libcrypto.so.*,/opt/apps/internal-test-*,테스트 앱 잡음\n" +
			"include,openssl,libcrypto.so.3,/opt/apps/payment-gw,운영 앱 — 공유 .so 보존\n"))
	if kept, excl := p2.Apply([]*discoveryv1.Finding{shared()}); len(kept) != 1 || excl != 0 {
		t.Fatalf("뒤 include로 운영 앱의 공유 .so가 구제돼야: kept=%d excl=%d", len(kept), excl)
	}

	// (3) 순서 의존성: include를 exclude '앞에' 두면 뒤의 exclude가 이긴다 — 무조건 우선이 아니다.
	p3, _ := scope.LoadAssetPolicy(strings.NewReader(
		"include,openssl,libcrypto.so.3,/opt/apps/payment-gw,\n" +
			"exclude,openssl,libcrypto.so.*,/opt/apps/internal-test-*,\n"))
	if kept, _ := p3.Apply([]*discoveryv1.Finding{shared()}); len(kept) != 0 {
		t.Errorf("include를 앞에 두면 뒤 exclude가 이긴다(last-wins, 무조건 우선 아님): kept=%d", len(kept))
	}
}

func TestBadAction(t *testing.T) {
	if _, err := scope.LoadAssetPolicy(strings.NewReader("drop,*,*,*,오타\n")); err == nil {
		t.Error("action 오타는 오류여야 함 — 조용히 무시하면 정책이 안 먹은 걸 모른다")
	}
}
