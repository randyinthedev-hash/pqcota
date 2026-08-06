package provisioning_test

import (
	"strings"
	"testing"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	provisioningv1 "github.com/pqcota/pqcota/gen/pqcota/provisioning/v1"
	"github.com/pqcota/pqcota/pkg/provisioning"
)

func action(rt commonv1.CryptoRuntime, kind provisioningv1.RemediationKind, target, provider string) *provisioningv1.RemediationAction {
	return &provisioningv1.RemediationAction{
		TargetNodeId:    "web-01",
		CryptoRuntime:   rt,
		Kind:            kind,
		TargetAlgorithm: target,
		ProviderChoice:  provider,
	}
}

// OpenSSL config-only (3.5+): 하이브리드 그룹 활성화만, provider 로드 없음(프로비저닝 설계 §4.1).
func TestRenderOpenSSLConfigOnly(t *testing.T) {
	art := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY, "ML-KEM (FIPS 203)", ""))
	for _, want := range []string{"[system_default_sect]", "Groups", "X25519MLKEM768"} {
		if !strings.Contains(art, want) {
			t.Errorf("config-only 조각에 %q 없음:\n%s", want, art)
		}
	}
	if strings.Contains(art, "module") {
		t.Errorf("config-only는 provider 로드(module) 없어야:\n%s", art)
	}
}

// OpenSSL provider-inject (3.0–3.4): provider 모듈 로드 + 활성화 + 그룹(프로비저닝 설계 §4.1).
func TestRenderOpenSSLProviderInject(t *testing.T) {
	art := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "ML-KEM (FIPS 203)", "oqsprovider"))
	for _, want := range []string{"provider_sect", "activate = 1", "module", "oqsprovider", "X25519MLKEM768"} {
		if !strings.Contains(art, want) {
			t.Errorf("provider-inject 조각에 %q 없음:\n%s", want, art)
		}
	}
}

// 목적: 생성한 조각을 OPENSSL_CONF로 직접 가리켜도 먹는 상태를 유지한다.
// [openssl_init]은 최상위 `openssl_conf = openssl_init`이 있어야 읽힌다 — 그 줄이 빠지면
// 배치도 되고 sha256 게이트도 통과하는데 provider는 안 올라오는, 가장 알아채기 어려운 실패가 된다
// (실물 oqsprovider로 재현했다). 섹션보다 **앞**이어야 하므로 위치까지 본다.
func TestRenderOpenSSLConfigIsUsableStandalone(t *testing.T) {
	kinds := map[string]provisioningv1.RemediationKind{
		"config-only":     provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY,
		"provider-inject": provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT,
	}
	for name, kind := range kinds {
		t.Run(name, func(t *testing.T) {
			art := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
				kind, "ML-KEM (FIPS 203)", ""))
			at := strings.Index(art, "openssl_conf = openssl_init")
			if at < 0 {
				t.Fatalf("최상위 지시자가 없다 — OpenSSL이 [openssl_init]을 읽지 않는다:\n%s", art)
			}
			if sect := strings.Index(art, "["); sect >= 0 && at > sect {
				t.Errorf("최상위 지시자가 첫 섹션(%d) 뒤(%d)에 있다 — 그 섹션에 속해 버린다:\n%s", sect, at, art)
			}
		})
	}
}

// config로 주입 불가한 조치(포크 교체 등)는 정직하게 비-config 조치임을 명시(프로비저닝 설계 §4.1 "레거시 터치").
func TestRenderOpenSSLNonConfig(t *testing.T) {
	art := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		provisioningv1.RemediationKind_REMEDIATION_KIND_FORK_REPLACE, "ML-KEM (FIPS 203)", ""))
	if !strings.Contains(art, "#") || strings.Contains(art, "[system_default_sect]") {
		t.Errorf("포크 교체는 config 조각이 아니라 주석 조치여야:\n%s", art)
	}
}

// JCA config-only (JDK 네이티브): namedGroups config만, provider 등록 없음(프로비저닝 설계 §4.2).
func TestRenderJCAConfigOnly(t *testing.T) {
	art := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY, "ML-KEM (FIPS 203)", "JDK-native"))
	if !strings.Contains(art, "jdk.tls.namedGroups") {
		t.Errorf("JCA config-only에 namedGroups 없음:\n%s", art)
	}
	if strings.Contains(art, "security.provider.") {
		t.Errorf("config-only는 provider 등록 없어야:\n%s", art)
	}
}

// JCA provider-inject: provider 클래스 등록 + JAR 배치 + namedGroups(프로비저닝 설계 §4.2). BC/BCFIPS 라우팅.
func TestRenderJCAProviderInject(t *testing.T) {
	bc := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "ML-KEM (FIPS 203)", "BC"))
	for _, want := range []string{"security.provider.", "BouncyCastleProvider", "jdk.tls.namedGroups"} {
		if !strings.Contains(bc, want) {
			t.Errorf("BC provider-inject에 %q 없음:\n%s", want, bc)
		}
	}
	fips := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "ML-KEM (FIPS 203)", "BCFIPS"))
	if !strings.Contains(fips, "BouncyCastleFipsProvider") {
		t.Errorf("BCFIPS는 FIPS provider 클래스여야:\n%s", fips)
	}
}

// FillPlan — 계획의 모든 조치에 config_artifact를 결정론적으로 채운다(§1.2).
func TestFillPlan(t *testing.T) {
	p := &provisioningv1.FinalizedPlan{Actions: []*provisioningv1.RemediationAction{
		action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL, provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY, "ML-KEM (FIPS 203)", ""),
		action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA, provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "ML-KEM (FIPS 203)", "BC"),
	}}
	provisioning.FillPlan(p)
	for i, a := range p.GetActions() {
		if a.GetConfigArtifact() == "" {
			t.Errorf("actions[%d] config_artifact 미채움", i)
		}
	}
}

// provider_class를 명시하면 커스텀 JCA provider도 계획만으로 완결된다(placeholder 없음).
func TestRenderJCAExplicitProviderClass(t *testing.T) {
	out := provisioning.Render(&provisioningv1.RemediationAction{
		CryptoRuntime:   commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		Kind:            provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT,
		TargetAlgorithm: "ML-KEM (FIPS 203)",
		ProviderChoice:  "acme-jce",
		ProviderClass:   "com.acme.jce.AcmeProvider",
	})
	if !strings.Contains(out, "security.provider.2=com.acme.jce.AcmeProvider") {
		t.Errorf("명시한 FQCN이 그대로 등록돼야 함:\n%s", out)
	}
	if strings.Contains(out, "placeholder") {
		t.Errorf("클래스명을 명시했는데 placeholder 경고가 남았다:\n%s", out)
	}
}

// 명시가 없고 알려지지 않은 이름이면 placeholder + 해결 방법을 안내한다(추측 금지).
func TestRenderJCAUnknownProviderKeepsPlaceholder(t *testing.T) {
	out := provisioning.Render(&provisioningv1.RemediationAction{
		CryptoRuntime:   commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		Kind:            provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT,
		TargetAlgorithm: "ML-KEM (FIPS 203)",
		ProviderChoice:  "acme-jce",
	})
	for _, want := range []string{"placeholder", "provider_class", "<acme-jce:"} {
		if !strings.Contains(out, want) {
			t.Errorf("미상 provider 안내에 %q 없음:\n%s", want, out)
		}
	}
}

// JAR 배치 안내는 실제 배치 경로와 JDK 세대 차이를 말해야 한다(lib/ext는 JDK 9에서 제거됨).
func TestRenderJCAJarPlacementGuidance(t *testing.T) {
	out := provisioning.Render(&provisioningv1.RemediationAction{
		CryptoRuntime:   commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		Kind:            provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT,
		TargetAlgorithm: "ML-KEM (FIPS 203)",
		ProviderChoice:  "BC",
	})
	if !strings.Contains(out, provisioning.ModulePath("BC", true)) {
		t.Errorf("실제 배치 경로가 안내에 없다:\n%s", out)
	}
	if !strings.Contains(out, "JDK 9+") {
		t.Errorf("JDK 9+ 안내가 없다 — lib/ext는 9에서 제거됐다:\n%s", out)
	}
}

// BC 기본 클래스의 정답은 **버전에 달렸다** — 실측: 1.80/1.81의 BouncyCastleProvider에는 ML-KEM
// 서비스가 17개, 1.78.1에는 0개(Kyber는 BouncyCastlePQCProvider에 따로). 계획은 JAR 버전을
// 말해주지 않으므로, 기본값을 쓰되 그 전제를 조각에 적는다 — 조용히 단언하지 않는다(§2.5).
func TestBCDefaultClassStatesVersionAssumption(t *testing.T) {
	bc := action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "ML-KEM (FIPS 203)", "BC")
	out := provisioning.Render(bc)
	for _, want := range []string{"1.80+", "BouncyCastlePQCProvider", "provider_class"} {
		if !strings.Contains(out, want) {
			t.Errorf("BC 기본값에 버전 전제(%q)가 없다:\n%s", want, out)
		}
	}

	// 계획이 클래스를 명시했으면 그건 저자의 결정이라 도구가 전제를 덧붙이지 않는다.
	bc.ProviderClass = "com.acme.AcmeProvider"
	if out := provisioning.Render(bc); strings.Contains(out, "1.80+") {
		t.Errorf("명시된 클래스에 BC 버전 전제가 붙었다:\n%s", out)
	}

	// BCFIPS는 다른 아티팩트라 BC 세대 문제와 무관하다.
	fips := action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "ML-KEM (FIPS 203)", "BCFIPS")
	if out := provisioning.Render(fips); strings.Contains(out, "1.80+") {
		t.Errorf("BCFIPS에 BC 버전 전제가 붙었다:\n%s", out)
	}
}

// 폴백 그룹은 장식이 아니다 — 실측: jdk.tls.namedGroups에 **미지 그룹만** 주면 JDK 21의 JSSE가
// 초기화 자체에 실패한다(ExceptionInInitializerError). 고전 그룹을 함께 두는 것이 앱의 TLS가
// 죽지 않게 하는 유일한 이유다. 이 줄이 사라지면 생성물이 앱을 망가뜨린다.
func TestNamedGroupsAlwaysKeepsClassicFallback(t *testing.T) {
	for _, kind := range []provisioningv1.RemediationKind{
		provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY,
		provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT,
	} {
		out := provisioning.Render(action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA, kind, "ML-KEM (FIPS 203)", "BC"))
		if !strings.Contains(out, "jdk.tls.namedGroups=X25519MLKEM768,x25519") {
			t.Errorf("%s: PQC 그룹만 남으면 JSSE가 뜨지 않는다 — 고전 폴백이 있어야:\n%s", kind, out)
		}
	}
}

// ★ `security.provider.2=`는 **끼워 넣지 않고 그 자리를 대체한다** — 실측: JDK 21에서 이 줄을
// 적용하면 provider 목록이 12개 그대로이고 원래 2번이던 SunRsaSign이 사라진다(RSA 서비스가 새
// provider로 넘어간다). 삽입처럼 읽히는 문구를 쓰면 생성물이 거짓말을 한다.
func TestProviderSlotReplacementIsStated(t *testing.T) {
	bc := action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "ML-KEM (FIPS 203)", "BC")
	out := provisioning.Render(bc)
	if !strings.Contains(out, "security.provider.2=") {
		t.Fatalf("등록 줄이 없다:\n%s", out)
	}
	for _, want := range []string{"대체한다", "SunRsaSign", "뒤 번호"} {
		if !strings.Contains(out, want) {
			t.Errorf("자리 대체 사실(%q)이 조각에 없다:\n%s", want, out)
		}
	}

	// 조각 안 주석은 열어봐야 보이므로 stderr 경고로도 나가야 한다.
	plan := &provisioningv1.FinalizedPlan{Actions: []*provisioningv1.RemediationAction{bc}}
	if w := provisioning.ProviderSlotWarnings(plan); len(w) != 1 || !strings.Contains(w[0], "대체한다") {
		t.Errorf("주입 조치에 자리 대체 경고가 있어야: %v", w)
	}
	// config-only는 provider를 등록하지 않으므로 경고 대상이 아니다.
	cfg := action(commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
		provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY, "ML-KEM (FIPS 203)", "")
	if w := provisioning.ProviderSlotWarnings(&provisioningv1.FinalizedPlan{
		Actions: []*provisioningv1.RemediationAction{cfg}}); len(w) != 0 {
		t.Errorf("config-only에 자리 경고가 나오면 안 된다: %v", w)
	}
}
