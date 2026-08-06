package provisioning

import (
	"fmt"
	"strings"

	provisioningv1 "github.com/pqcota/pqcota/gen/pqcota/provisioning/v1"
)

// renderJCA — JCA/JCE remediation taxonomy(§4.4) → java.security 조각.
//   - CONFIG_ONLY (JDK 네이티브 PQC): namedGroups·순서 config만. provider 무등록.
//   - PROVIDER_INJECT (provider JAR) : provider 클래스 등록 + JAR 배치 + namedGroups.
//   - 그 외(앱 reconfig·JDK 업그레이드·재빌드·폐기): config로 안 되는 조치 → 주석 블록.
func renderJCA(a *provisioningv1.RemediationAction) string {
	group := hybridGroup(a.GetTargetAlgorithm())
	switch a.GetKind() {
	case provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY:
		return jcaConfigOnly(group, a.GetTargetAlgorithm())
	case provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT:
		return jcaProviderInject(group, a.GetTargetAlgorithm(), a.GetProviderChoice(), a.GetProviderClass())
	default:
		return jcaNonConfig(a)
	}
}

// providerClass — provider 라우팅(§4.4) → java.security 등록 클래스명.
//
// 계획이 provider_class를 명시했으면 그대로 쓴다 — 커스텀 provider가 계획만으로 완결되는 경로다.
// 명시가 없으면 알려진 이름(BC/BCFIPS)만 확정하고, 그 외는 placeholder로 두고 provider 문서
// 확인을 명시한다(추측해서 채우지 않는다).
//
// ★ BC의 정답은 **버전에 달렸다**(실측): 1.80·1.81의 BouncyCastleProvider에는 ML-KEM 서비스가
// 17개 있으나 1.78.1에는 **0개**이고 Kyber가 BouncyCastlePQCProvider에 따로 있다. 기본값은 현재
// 통용되는 1.80+ 기준으로 두되, 그 전제를 조각에 적어 둔다 — 계획이 JAR 버전을 말해주지 않으므로
// 도구가 대신 안다고 할 수 없다(§2.6).
func providerClass(choice, explicit string) (class string, exact bool) {
	if explicit != "" {
		return explicit, true
	}
	switch choice {
	case "BC":
		return "org.bouncycastle.jce.provider.BouncyCastleProvider", true
	case "BCFIPS", "BC-FJA":
		return "org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider", true
	default:
		if choice == "" {
			choice = "BC"
			return "org.bouncycastle.jce.provider.BouncyCastleProvider", true
		}
		return fmt.Sprintf("<%s: provider 문서의 정식 클래스명 확인>", choice), false
	}
}

// namedGroupsLine — JDK TLS 협상 그룹(PQC 우선, 고전 폴백으로 하위호환).
func namedGroupsLine(group string) string {
	if group == "" {
		return "# jdk.tls.namedGroups: 목표가 KEM 하이브리드 그룹이 아님(서명·미상) — 수동 지정\n"
	}
	return fmt.Sprintf("jdk.tls.namedGroups=%s,x25519\n", group)
}

func jcaConfigOnly(group, target string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# pqcota 생성: JDK 네이티브 PQC config-only — %s(§4.4)\n", target)
	b.WriteString("# provider 무등록. 네이티브 지원 그룹만 협상에 포함. 롤백=이 조각 제거.\n")
	b.WriteString(namedGroupsLine(group))
	return b.String()
}

func jcaProviderInject(group, target, choice, explicitClass string) string {
	class, exact := providerClass(choice, explicitClass)
	var b strings.Builder
	fmt.Fprintf(&b, "# pqcota 생성: JCA provider 주입 — %s (provider=%s)(§4.4)\n", target, choice)
	b.WriteString("# JAR 배치 후 java.security에 등록. 재배포 없이 provider 보강. 롤백=등록 라인 제거.\n")
	if !exact {
		b.WriteString("# ⚠ 아래 클래스명은 placeholder — provider 배포본의 정식 클래스로 교체하거나\n")
		b.WriteString("#   계획의 provider_class에 FQCN을 넣으면 자동으로 채워진다.\n")
	}
	// 계획이 클래스를 명시하지 않아 BC 기본값을 쓴 경우에만 — 명시했다면 그건 저자의 결정이다.
	if explicitClass == "" && (choice == "BC" || choice == "") {
		b.WriteString("# ⚠ 전제: BouncyCastle **1.80+**. 1.78.x 이하는 BouncyCastleProvider에 KEM이 없고\n")
		b.WriteString("#   Kyber가 org.bouncycastle.pqc.jcajce.provider.BouncyCastlePQCProvider에 따로 있다.\n")
		b.WriteString("#   배치할 JAR이 그 세대면 계획의 provider_class에 그 클래스를 명시할 것.\n")
	}
	// JAR은 플레이북이 여기 놓는다. JVM이 찾게 하려면 클래스패스에 얹어야 하는데, 그 방법이
	// JDK 세대마다 다르다 — 확장 메커니즘(lib/ext)은 JDK 9에서 제거됐다.
	fmt.Fprintf(&b, "# JAR 배치: %s (플레이북이 여기 놓는다)\n", ModulePath(choice, true))
	b.WriteString("#   JDK 8  : $JAVA_HOME/jre/lib/ext 에 두거나 앱 classpath에 포함\n")
	b.WriteString("#   JDK 9+ : lib/ext 없음 — classpath 또는 --module-path 에 포함시킬 것\n")
	// 우선순위 N: 디스패치 체인 진입 보장 위해 상위(2)에 둔다. 전역 변경이라 blast radius가 있다(§4.4).
	//
	// ★ 이 줄은 **끼워 넣지 않고 그 자리를 차지한다**(실측: JDK 21에서 provider.2를 이 값으로 두면
	// 원래 2번이던 SunRsaSign이 목록에서 사라진다 — 12개가 12개 그대로이고 이름만 바뀐다). 삽입처럼
	// 읽히게 적으면 거짓말이 되므로 조각이 직접 그렇게 말한다. 밀어내지 않고 넣으려면 뒤 번호를 전부
	// 한 칸씩 미뤄야 하는데, 그러려면 그 노드의 java.security 원본을 알아야 한다 — 도구는 모른다(§2.6).
	fmt.Fprintf(&b, "security.provider.2=%s\n", class)
	b.WriteString("# ↑ 우선순위 2 — PQC 알고리즘이 먼저 디스패치되게(수용 원칙 §2.2(d) 순서 협상).\n")
	b.WriteString("# ⚠ 이 줄은 **2번 자리를 대체한다**(끼워 넣지 않는다). 원래 2번이던 provider는\n")
	b.WriteString("#   목록에서 빠진다 — JDK 기본값이면 대개 SunRsaSign이고, 그러면 RSA 서비스가\n")
	b.WriteString("#   이 provider 구현으로 넘어간다. 밀어내지 않으려면 대상 노드의 java.security에서\n")
	b.WriteString("#   뒤 번호를 한 칸씩 미룬 뒤 이 줄을 넣을 것.\n")
	b.WriteString(namedGroupsLine(group))
	return b.String()
}

// jcaNonConfig — java.security 조각으로 해결 불가한 조치(§4.4 "레거시 터치").
func jcaNonConfig(a *provisioningv1.RemediationAction) string {
	var reason string
	switch a.GetKind() {
	case provisioningv1.RemediationKind_REMEDIATION_KIND_APP_RECONFIG:
		reason = "앱이 getInstance(...,\"BC\") 명시 지목·그룹 고정 — 앱 코드·설정 변경 필요."
	case provisioningv1.RemediationKind_REMEDIATION_KIND_JDK_UPGRADE:
		reason = "EOL JDK — JDK 업그레이드 또는 프록시 프론팅. 레거시 터치=필요."
	case provisioningv1.RemediationKind_REMEDIATION_KIND_REBUILD:
		reason = "셰이딩(shaded) 크립토 — CI 재빌드 필요. 레거시 터치=필요."
	case provisioningv1.RemediationKind_REMEDIATION_KIND_DECOMMISSION:
		reason = "EOL·저가치 — 폐기 또는 리스크 수용."
	default:
		reason = "config로 주입 불가한 조치."
	}
	return fmt.Sprintf("# pqcota: JCA 비-config 조치(%s)\n# %s\n# → 이 조치는 java.security 조각으로 생성되지 않는다. 계획 수동 단계 참조.\n",
		a.GetKind(), reason)
}
