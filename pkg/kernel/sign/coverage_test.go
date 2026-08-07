package sign_test

import (
	"testing"
	"time"

	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/kernel/sign"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 서명이 실제로 무엇을 덮는지 고정한다. 덮이지 않는 필드는 **변조해도 검증이 통과**하므로,
// "서명했다"는 말이 실제보다 강하게 들리게 된다. 각 필드를 흔들어 검증이 깨지는지 본다.

func signed(t *testing.T) (pub string, res *discoveryv1.CollectionResult) {
	t.Helper()
	pub, priv, err := sign.Generate()
	if err != nil {
		t.Fatal(err)
	}
	ts := timestamppb.New(time.Unix(1700000000, 0).UTC())
	res = &discoveryv1.CollectionResult{
		Envelope: &commonv1.Envelope{
			CollectorId: "openssl-collector", CollectorVersion: "0.1.0",
			DetectionMethod: commonv1.DetectionMethod_DETECTION_METHOD_RUNTIME_INTROSPECTION,
			CollectedAt:     ts, TargetNodeId: "node-a", ScopeMasterRef: "cmdb-42",
			CollectorLicense: "Apache-2.0",
			Machine: &commonv1.MachineIdentity{
				MachineId: "m-1", HardwareUuid: "hw-1", CloudInstanceId: "i-1",
				Fqdn: "a.example", Ips: []string{"10.0.0.1"},
			},
		},
		RawCapture: []byte("native-output"), RawFormat: "openssl-collector/native-v1",
		CbomCyclonedx: []byte(`{"bomFormat":"CycloneDX"}`), CyclonedxSpecVersion: "1.6",
		Completeness: &commonv1.Completeness{
			LayersCovered: []commonv1.CollectionLayer{commonv1.CollectionLayer_COLLECTION_LAYER_PROCESS},
			LayersMissing: []commonv1.CollectionLayer{commonv1.CollectionLayer_COLLECTION_LAYER_NETWORK},
			Note:          "네트워크 계층 미수집",
		},
		ObservedEdges: []*discoveryv1.ObservedEdge{{
			SrcNodeId: "node-a", DstNodeId: "node-b", DstAddr: "10.0.0.2:443", Port: 443,
			Protocol:        discoveryv1.NetworkProtocol_NETWORK_PROTOCOL_TLS,
			Role:            discoveryv1.EdgeRole_EDGE_ROLE_CLIENT,
			NegotiatedGroup: "X25519MLKEM768", Cipher: "TLS_AES_128_GCM_SHA256",
			DetectionMethod: commonv1.DetectionMethod_DETECTION_METHOD_RUNTIME_INTROSPECTION,
			ObservedCount:   7, FirstSeen: ts, LastSeen: ts,
		}},
	}
	sig, err := sign.Sign(priv, res)
	if err != nil {
		t.Fatal(err)
	}
	res.Envelope.Signature = sig
	if !sign.Verify([]string{pub}, res) {
		t.Fatal("서명 직후 검증이 실패하면 안 된다")
	}
	return pub, res
}

// 각 필드를 변조하면 검증이 깨져야 한다. 특히 completeness(갭 선언)와 raw_capture(재계산 원본).
func TestTamperBreaksVerification(t *testing.T) {
	cases := map[string]func(*discoveryv1.CollectionResult){
		"envelope.collector_id":      func(r *discoveryv1.CollectionResult) { r.Envelope.CollectorId = "x" },
		"envelope.collector_version": func(r *discoveryv1.CollectionResult) { r.Envelope.CollectorVersion = "9" },
		"envelope.detection_method": func(r *discoveryv1.CollectionResult) {
			r.Envelope.DetectionMethod = commonv1.DetectionMethod_DETECTION_METHOD_SYMBOL_ANALYSIS
		},
		"envelope.collected_at":     func(r *discoveryv1.CollectionResult) { r.Envelope.CollectedAt = timestamppb.Now() },
		"envelope.target_node_id":   func(r *discoveryv1.CollectionResult) { r.Envelope.TargetNodeId = "node-z" },
		"envelope.scope_master_ref": func(r *discoveryv1.CollectionResult) { r.Envelope.ScopeMasterRef = "cmdb-99" },
		"envelope.collector_license": func(r *discoveryv1.CollectionResult) {
			r.Envelope.CollectorLicense = "GPL-3.0-or-later"
		},
		"envelope.machine.machine_id": func(r *discoveryv1.CollectionResult) { r.Envelope.Machine.MachineId = "m-9" },
		"envelope.machine.ips":        func(r *discoveryv1.CollectionResult) { r.Envelope.Machine.Ips = []string{"10.0.0.9"} },
		"raw_capture":                 func(r *discoveryv1.CollectionResult) { r.RawCapture = []byte("swapped") },
		"raw_format":                  func(r *discoveryv1.CollectionResult) { r.RawFormat = "other/v1" },
		"cbom_cyclonedx":              func(r *discoveryv1.CollectionResult) { r.CbomCyclonedx = []byte(`{"bomFormat":"x"}`) },
		"cyclonedx_spec_version":      func(r *discoveryv1.CollectionResult) { r.CyclonedxSpecVersion = "1.7" },
		// ★ 갭 선언 제거 — "원리상 관측하지 못했다"를 "없다"로 바꾸는 변조(§2.6). 반드시 잡혀야 한다.
		"completeness.layers_missing 제거": func(r *discoveryv1.CollectionResult) { r.Completeness.LayersMissing = nil },
		"completeness.note":              func(r *discoveryv1.CollectionResult) { r.Completeness.Note = "" },
		"completeness 통째 제거":             func(r *discoveryv1.CollectionResult) { r.Completeness = nil },
		"edge.dst_node_id":               func(r *discoveryv1.CollectionResult) { r.ObservedEdges[0].DstNodeId = "node-z" },
		"edge.port":                      func(r *discoveryv1.CollectionResult) { r.ObservedEdges[0].Port = 8443 },
		"edge.negotiated_group": func(r *discoveryv1.CollectionResult) {
			r.ObservedEdges[0].NegotiatedGroup = "x25519" // 🟢 → 🔴 등급을 뒤집는 변조
		},
		"edge.cipher":         func(r *discoveryv1.CollectionResult) { r.ObservedEdges[0].Cipher = "NULL" },
		"edge.role":           func(r *discoveryv1.CollectionResult) { r.ObservedEdges[0].Role = discoveryv1.EdgeRole_EDGE_ROLE_SERVER },
		"edge.observed_count": func(r *discoveryv1.CollectionResult) { r.ObservedEdges[0].ObservedCount = 1 },
		"edge.last_seen":      func(r *discoveryv1.CollectionResult) { r.ObservedEdges[0].LastSeen = timestamppb.Now() },
		"edge 추가": func(r *discoveryv1.CollectionResult) {
			r.ObservedEdges = append(r.ObservedEdges, &discoveryv1.ObservedEdge{SrcNodeId: "node-a", DstAddr: "10.0.0.3:22"})
		},
		"edge 제거": func(r *discoveryv1.CollectionResult) { r.ObservedEdges = nil },
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			pub, res := signed(t)
			tamper(res)
			if sign.Verify([]string{pub}, res) {
				t.Errorf("%s 를 변조했는데 검증이 통과했다 — 이 필드가 서명 사각지대다", name)
			}
		})
	}
}

// 엣지 순서가 달라도 같은 관측 집합이면 검증이 통과해야 한다(정렬 정규화).
func TestEdgeOrderDoesNotMatter(t *testing.T) {
	pub, res := signed(t)
	res.ObservedEdges = append(res.ObservedEdges, &discoveryv1.ObservedEdge{
		SrcNodeId: "node-a", DstAddr: "10.0.0.3:22",
		Protocol: discoveryv1.NetworkProtocol_NETWORK_PROTOCOL_SSH,
	})
	// 다시 서명한 뒤 순서만 뒤집는다.
	_, priv, _ := sign.Generate()
	_ = priv
	pub2, priv2, _ := sign.Generate()
	sig, _ := sign.Sign(priv2, res)
	res.Envelope.Signature = sig
	res.ObservedEdges[0], res.ObservedEdges[1] = res.ObservedEdges[1], res.ObservedEdges[0]
	if !sign.Verify([]string{pub2}, res) {
		t.Error("엣지 순서만 바뀌었는데 검증이 실패했다 — 정규화가 순서에 흔들린다")
	}
	_ = pub
}

// ★ 필드 수 가드 — 계약에 필드가 늘면 여기서 실패한다. Canonical을 함께 갱신하라는 신호다.
// (서명 사각지대는 조용히 생기므로, 소리 나게 만든다.)
func TestCanonicalCoversAllFields(t *testing.T) {
	want := map[string]int{
		"CollectionResult": 7,  // envelope, raw_capture, raw_format, cbom_cyclonedx, spec_version, completeness, observed_edges
		"Envelope":         9,  // 그중 signature는 서명 대상에서 제외(자기 자신)
		"MachineIdentity":  7,  // machine_id, hardware_uuid, cloud_instance_id, fqdn, ips, self_assigned_id, derived_from
		"Completeness":     3,  // layers_covered, layers_missing, note
		"ObservedEdge":     12, // src, dst_node, dst_addr, port, proto, role, group, cipher, detection, count, first, last
	}
	got := map[string]int{
		"CollectionResult": (&discoveryv1.CollectionResult{}).ProtoReflect().Descriptor().Fields().Len(),
		"Envelope":         (&commonv1.Envelope{}).ProtoReflect().Descriptor().Fields().Len(),
		"MachineIdentity":  (&commonv1.MachineIdentity{}).ProtoReflect().Descriptor().Fields().Len(),
		"Completeness":     (&commonv1.Completeness{}).ProtoReflect().Descriptor().Fields().Len(),
		"ObservedEdge":     (&discoveryv1.ObservedEdge{}).ProtoReflect().Descriptor().Fields().Len(),
	}
	for msg, n := range want {
		if got[msg] != n {
			t.Errorf("%s 필드 수가 %d → %d로 바뀌었다.\n"+
				"  계약이 바뀌면 sign.Canonical도 함께 갱신해야 한다 — 안 그러면 새 필드가 서명 사각지대가 된다.\n"+
				"  갱신했다면 이 기대값도 %d로 고치고, TestTamperBreaksVerification에 케이스를 추가하라.\n"+
				"  주의: Canonical 범위를 바꾸면 기존 서명은 전부 무효가 된다.", msg, n, got[msg], got[msg])
		}
	}
}
