package openssl

import (
	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/discovery/normalize"
)

// BuildResult — 탐지 결과를 정규화된 CBOM Envelope(CollectionResult)로. 노드 단위 집계에 쓴다.
// dets가 비면 프로세스 계층 미커버(갭). CycloneDX + pqcota properties(§3.2).
func BuildResult(node string, dets []Detection) *discoveryv1.CollectionResult {
	declared := []commonv1.CollectionLayer{
		commonv1.CollectionLayer_COLLECTION_LAYER_PROCESS,
		commonv1.CollectionLayer_COLLECTION_LAYER_ARTIFACT,
	}
	var covered []commonv1.CollectionLayer
	var cyclone []byte
	note := "OpenSSL 미검출 또는 접근 불가"
	if len(dets) > 0 {
		covered = []commonv1.CollectionLayer{commonv1.CollectionLayer_COLLECTION_LAYER_PROCESS}
		cyclone, _ = buildCycloneDX(dets)
		note = ""
	}
	// 원본이 없으면 형식 이름도 비운다(§1.2 — 재정규화할 것이 없는데 있다고 하지 않는다).
	raw := RawCapture(dets)
	rawFormat := "openssl-collector/native-v1"
	if len(raw) == 0 {
		rawFormat = ""
	}
	return &discoveryv1.CollectionResult{
		Envelope: &commonv1.Envelope{
			CollectorId:      "openssl-collector",
			CollectorVersion: "0.1.0",
			DetectionMethod:  commonv1.DetectionMethod_DETECTION_METHOD_RUNTIME_INTROSPECTION,
			TargetNodeId:     node,
			CollectorLicense: "Apache-2.0",
		},
		RawCapture:           raw,
		RawFormat:            rawFormat,
		CbomCyclonedx:        cyclone,
		CyclonedxSpecVersion: "1.6",
		Completeness:         normalize.BuildCompleteness(declared, covered, note),
	}
}
