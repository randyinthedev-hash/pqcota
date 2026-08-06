// Package jvm hosts the JVM collector's Go side: it parses the Java attach
// sidecar 출력을 정규화된 CBOM Envelope(CollectionResult)로 변환하고 intake 계약(§6.1)으로 노출한다.
// (attach 자체는 순수 Java 사이드카 — discovery/collectors/jvm/collector. 여기선 결과를 계약으로.)
package jvm

import (
	"encoding/json"
	"strconv"
	"strings"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
)

// Provider — getProviders() 실체의 한 항목(등록 순서 포함, 수용 원칙 §2.2).
type Provider struct {
	Order     int
	Name      string
	Version   string
	ClassName string
}

// Collected — 사이드카 출력 파싱 결과. Degraded=true면 attach 불가로 정적 폴백(설계 §2.2, TD-JVM-4).
type Collected struct {
	Providers []Provider
	Degraded  bool
	// Raw — 파싱 전 원본 텍스트. raw_capture로 실려 재정규화의 입력이 된다(§2.5 step 1).
	Raw string
}

// ParseProviders — Java 사이드카 출력("N|name|version|class" 또는 정적 폴백)을 파싱.
func ParseProviders(output string) Collected {
	c := Collected{Degraded: strings.Contains(output, "PQCOTA_STATIC_FALLBACK_BEGIN"), Raw: output}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PQCOTA_") ||
			strings.HasPrefix(line, "evidence_strength=") || strings.HasPrefix(line, "gap=") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		ord, _ := strconv.Atoi(parts[0])
		p := Provider{Order: ord, Name: parts[1]}
		if len(parts) >= 3 {
			p.Version = parts[2]
		}
		if len(parts) >= 4 {
			p.ClassName = parts[3]
		}
		c.Providers = append(c.Providers, p)
	}
	return c
}

// BuildResult — ident 없는 단일 JVM 편의 형태. 다중 JVM은 BuildResultFor로 구별한다.
func BuildResult(node string, c Collected) *discoveryv1.CollectionResult {
	return BuildResultFor(node, c, "")
}

// BuildResultFor — 파싱 결과를 정규화된 CBOM Envelope로. attach 성공은 runtime-introspection(confirmed 근거),
// 정적 폴백은 artifact + 완전성 갭(§2.6 갭≠부재). provider_set 순서 보존(수용 원칙 §2.2).
//
// ident: 한 노드에 JVM이 여럿일 때 이들을 **구별**하는 안정 식별자(JAVA_HOME 권장). 비면 단일.
// finding id는 (node|컴포넌트명|runtime|fork) 해시라, ident를 컴포넌트명에 실어야 서로 다른 JVM이
// 하나로 뭉개지지 않는다. **PID는 쓰지 않는다** — 매 스캔 달라져 이력이 "매번 새 자산"으로 깨진다.
func BuildResultFor(node string, c Collected, ident string) *discoveryv1.CollectionResult {
	dm := commonv1.DetectionMethod_DETECTION_METHOD_RUNTIME_INTROSPECTION
	dmStr := "runtime-introspection"
	covered := []commonv1.CollectionLayer{commonv1.CollectionLayer_COLLECTION_LAYER_JVM_INTROSPECTION}
	var missing []commonv1.CollectionLayer
	note := ""
	if c.Degraded {
		dm = commonv1.DetectionMethod_DETECTION_METHOD_ARTIFACT
		dmStr = "artifact"
		covered = nil
		missing = []commonv1.CollectionLayer{commonv1.CollectionLayer_COLLECTION_LAYER_JVM_INTROSPECTION}
		note = "attach 불가 — java.security 정적 경로(runtime-introspection 갭, 동적 등록 사각지대)"
	}

	names := make([]string, 0, len(c.Providers))
	for _, p := range c.Providers {
		names = append(names, p.Name)
	}

	compName := "jca-provider-chain"
	var appKeys []string
	if ident != "" { // 다중 JVM 구별 — 컴포넌트명·귀속에 안정 식별자
		compName += "@" + ident
		appKeys = []string{ident}
	}

	return &discoveryv1.CollectionResult{
		Envelope: &commonv1.Envelope{
			CollectorId:      "jvm-collector",
			CollectorVersion: "0.1.0",
			DetectionMethod:  dm,
			TargetNodeId:     node,
			CollectorLicense: "Apache-2.0",
		},
		RawCapture:           []byte(c.Raw),
		RawFormat:            "jvm-collector/providers-v1",
		CbomCyclonedx:        buildJcaCycloneDX(names, dmStr, compName, appKeys),
		CyclonedxSpecVersion: "1.6",
		Completeness: &commonv1.Completeness{
			LayersCovered: covered,
			LayersMissing: missing,
			Note:          note,
		},
	}
}

// buildJcaCycloneDX — provider 체인을 CycloneDX 본문 + pqcota properties(§3.2)로.
// provider_set은 등록 순서 CSV(수용 원칙 §2.2 우선순위 협상 판정 근거). compName·appKeys로 다중 JVM 구별.
func buildJcaCycloneDX(providerNames []string, detectionMethod, compName string, appKeys []string) []byte {
	type prop struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type comp struct {
		Type       string `json:"type"`
		Name       string `json:"name"`
		Properties []prop `json:"properties,omitempty"`
	}
	type doc struct {
		BomFormat   string `json:"bomFormat"`
		SpecVersion string `json:"specVersion"`
		Components  []comp `json:"components"`
	}
	props := []prop{
		{"pqcota:crypto_runtime", "jca"},
		{"pqcota:detection_method", detectionMethod},
		{"pqcota:jca.provider_set", strings.Join(providerNames, ",")},
		{"pqcota:jca.registration_mode", "dynamic"},
	}
	if len(appKeys) > 0 { // 다중 JVM 귀속(§0.5와 같은 결) — 어느 JDK/앱의 체인인지
		props = append(props, prop{"pqcota:app_keys", strings.Join(appKeys, ",")})
	}
	c := comp{Type: "cryptographic-asset", Name: compName, Properties: props}
	b, _ := json.Marshal(doc{BomFormat: "CycloneDX", SpecVersion: "1.6", Components: []comp{c}})
	return b
}
