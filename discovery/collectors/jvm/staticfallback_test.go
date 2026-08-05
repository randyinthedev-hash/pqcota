package jvm_test

import (
	"testing"

	"github.com/pqcota/pqcota/discovery/collectors/jvm"
)

// java.security 파싱 — 정적 등록 provider를 **N 순서대로**. 순서가 곧 우선순위라(§1.2)
// 파일에 적힌 줄 순서가 아니라 숫자 순이 맞다.
func TestParseJavaSecurity(t *testing.T) {
	// 일부러 줄 순서를 뒤섞고, 주석·빈 줄·인자 붙은 값·무관한 키를 섞는다.
	content := `# 표준 JDK의 java.security 발췌
security.provider.3=SunEC
security.provider.1=SUN

security.provider.2=SunRsaSign
# 아래는 인자가 붙는 형태
security.provider.4=SunPKCS11 ${java.home}/conf/security/pkcs11.cfg
securerandom.source=file:/dev/random
security.provider.5=
`
	c := jvm.ParseJavaSecurity(content)
	if !c.Degraded {
		t.Error("정적 경로는 항상 강등이어야(동적 등록 사각)")
	}
	want := []string{"SUN", "SunRsaSign", "SunEC", "SunPKCS11"}
	if len(c.Providers) != len(want) {
		t.Fatalf("provider %d개, want %d: %+v", len(c.Providers), len(want), c.Providers)
	}
	for i, w := range want {
		if c.Providers[i].Name != w {
			t.Errorf("[%d] = %q, want %q (N 순서대로여야)", i, c.Providers[i].Name, w)
		}
		if c.Providers[i].Order != i+1 {
			t.Errorf("[%d] Order = %d, want %d", i, c.Providers[i].Order, i+1)
		}
	}
}

// 빈 파일·provider 없음도 오류가 아니다 — 빈 목록 + 강등이 정직한 관측이다(§2.6).
func TestParseJavaSecurityEmpty(t *testing.T) {
	c := jvm.ParseJavaSecurity("# 주석뿐\nsecurerandom.source=file:/dev/random\n")
	if len(c.Providers) != 0 {
		t.Errorf("provider가 없어야: %+v", c.Providers)
	}
	if !c.Degraded {
		t.Error("빈 결과여도 강등 표시는 남아야 — '없다'가 아니라 '이 경로에선 못 본다'")
	}
}

// JAVA_HOME을 못 짚었으면 파일 위치를 모른다 — 지어내지 않고 오류 + 강등으로 돌려준다.
func TestStaticFallbackNoJavaHome(t *testing.T) {
	c, err := jvm.StaticFallbackGo(1, "")
	if err == nil {
		t.Error("JAVA_HOME 미상이면 오류여야")
	}
	if !c.Degraded {
		t.Error("실패해도 강등 표시는 있어야(조용한 0 금지)")
	}
}
