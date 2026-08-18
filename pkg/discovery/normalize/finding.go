package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/kernel/registry"
)

// cyclonedxDoc — 파싱에 필요한 최소 CycloneDX 형태(pqcota properties 포함).
type cyclonedxDoc struct {
	Components []struct {
		Name       string `json:"name"`
		Properties []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"properties"`
	} `json:"components"`
}

// DeriveFindings — 정규화 파이프라인 강화 단계(§2.4③). CollectionResult의 표준 CycloneDX +
// Envelope를 타입드 Finding으로 파생한다. evidence_strength는 detection_method에서 결정론적으로
// 부착되고(§2.5), 파생 뷰이므로 원본 재현을 위해 snapshotID·rulesetVersion을 기록한다(§1.2).
//
// ★ 이 강화는 코어 단독 책임 — Collector는 강화하지 않는다(설계 §3, contracts/README).
func DeriveFindings(res *discoveryv1.CollectionResult, snapshotID, rulesetVersion string) ([]*discoveryv1.Finding, error) {
	if len(res.GetCbomCyclonedx()) == 0 {
		return nil, nil
	}
	var doc cyclonedxDoc
	if err := json.Unmarshal(res.GetCbomCyclonedx(), &doc); err != nil {
		return nil, err
	}
	node := res.GetEnvelope().GetTargetNodeId()
	var out []*discoveryv1.Finding
	for _, c := range doc.Components {
		props := map[string]string{}
		for _, p := range c.Properties {
			props[p.Name] = p.Value
		}

		dm := parseDetectionMethod(props["pqcota:detection_method"], res.GetEnvelope().GetDetectionMethod())
		runtime := parseCryptoRuntime(props["pqcota:crypto_runtime"])

		f := &discoveryv1.Finding{
			CryptoRuntime:         runtime,
			DetectionMethod:       dm,
			EvidenceStrength:      EvidenceStrength(dm), // 결정론적 파생
			DerivedFromSnapshotId: snapshotID,
			RulesetVersion:        rulesetVersion,
		}

		switch runtime {
		case commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL:
			f.RuntimeAxes = &discoveryv1.Finding_Openssl{Openssl: &discoveryv1.OpensslAxes{
				Lib:         c.Name,
				Fork:        props["pqcota:openssl.fork"],
				Version:     props["pqcota:openssl.version"],
				BindingMode: parseBindingMode(props["pqcota:openssl.binding_mode"]),
			}}
		case commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA:
			providerSet := splitCSV(props["pqcota:jca.provider_set"])
			f.RuntimeAxes = &discoveryv1.Finding_Jca{Jca: &discoveryv1.JcaAxes{
				ProviderSet:      providerSet,
				RegistrationMode: parseRegistrationMode(props["pqcota:jca.registration_mode"]),
			}}
			// provider 시그니처 레지스트리 강화(수용 원칙 §2.3 · 규정서 §4.10): FIPS·SLH-DSA 갭.
			f.FipsValidation, f.PqcReadiness = jcaEnrichment(providerSet)
		}

		f.AppKeys = splitCSV(props["pqcota:app_keys"]) // 자산이 어느 앱 것인지(§1.5) — 어느 앱(들)의 크립토인가
		f.Id = findingID(node, c.Name, props)
		out = append(out, f)
	}
	return out, nil
}

// findingID — finding 동일성 정규화 해시(§2.4⑤ dedup 앵커). 노드+컴포넌트+런타임+fork 기반.
func findingID(node, name string, props map[string]string) string {
	key := strings.Join([]string{
		node, name,
		props["pqcota:crypto_runtime"],
		props["pqcota:openssl.fork"],
	}, "|")
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func parseCryptoRuntime(s string) commonv1.CryptoRuntime {
	switch s {
	case "openssl":
		return commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL
	case "jca":
		return commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA
	default:
		return commonv1.CryptoRuntime_CRYPTO_RUNTIME_UNSPECIFIED
	}
}

// parseDetectionMethod — 문자열(복합 가능, 예 "runtime-introspection+symbol-analysis")에서
// 가장 강한 방법을 택한다. 문자열이 비면 Envelope의 방법으로 폴백.
func parseDetectionMethod(s string, envelope commonv1.DetectionMethod) commonv1.DetectionMethod {
	switch {
	case strings.Contains(s, "runtime-introspection"):
		return commonv1.DetectionMethod_DETECTION_METHOD_RUNTIME_INTROSPECTION
	case strings.Contains(s, "dynamic-trace"):
		return commonv1.DetectionMethod_DETECTION_METHOD_DYNAMIC_TRACE
	case strings.Contains(s, "source"):
		return commonv1.DetectionMethod_DETECTION_METHOD_SOURCE
	case strings.Contains(s, "artifact"):
		return commonv1.DetectionMethod_DETECTION_METHOD_ARTIFACT
	case strings.Contains(s, "symbol-analysis"):
		return commonv1.DetectionMethod_DETECTION_METHOD_SYMBOL_ANALYSIS
	default:
		return envelope
	}
}

func parseBindingMode(s string) commonv1.OpensslBindingMode {
	switch s {
	case "dynamic":
		return commonv1.OpensslBindingMode_OPENSSL_BINDING_MODE_DYNAMIC
	case "static":
		return commonv1.OpensslBindingMode_OPENSSL_BINDING_MODE_STATIC
	case "dlopen":
		return commonv1.OpensslBindingMode_OPENSSL_BINDING_MODE_DLOPEN
	case "vendored":
		return commonv1.OpensslBindingMode_OPENSSL_BINDING_MODE_VENDORED
	default:
		return commonv1.OpensslBindingMode_OPENSSL_BINDING_MODE_UNSPECIFIED
	}
}

func parseRegistrationMode(s string) commonv1.JcaRegistrationMode {
	switch s {
	case "static":
		return commonv1.JcaRegistrationMode_JCA_REGISTRATION_MODE_STATIC
	case "dynamic":
		return commonv1.JcaRegistrationMode_JCA_REGISTRATION_MODE_DYNAMIC
	case "explicit":
		return commonv1.JcaRegistrationMode_JCA_REGISTRATION_MODE_EXPLICIT
	default:
		return commonv1.JcaRegistrationMode_JCA_REGISTRATION_MODE_UNSPECIFIED
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// jcaEnrichment — provider_set을 시그니처 레지스트리와 대조해 fips_validation·pqc_readiness를
// 파생한다(수용 원칙 §2.3 · 규정서 §4.10). SLH-DSA는 JDK 네이티브에 없으므로 갭을 명시한다.
func jcaEnrichment(providers []string) (fips, readiness string) {
	fips = "none"
	hasMLKEM, hasSLHDSA := false, false
	for _, name := range providers {
		if sig, ok := registry.MatchProvider(name, registry.DefaultProviderSignatures); ok {
			if sig.FipsValidation == "140-3" {
				fips = "140-3"
			}
			if sig.Covers("ML-KEM") {
				hasMLKEM = true
			}
			if sig.Covers("SLH-DSA") {
				hasSLHDSA = true
			}
		}
	}
	switch {
	case hasMLKEM && hasSLHDSA:
		readiness = "provider-보강(전 표준 알고리즘)"
	case hasMLKEM && !hasSLHDSA:
		readiness = "provider-보강(SLH-DSA 갭)"
	default:
		readiness = "unknown"
	}
	return fips, readiness
}
