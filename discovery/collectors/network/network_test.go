package network_test

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/pqcota/pqcota/discovery/collectors/network"
	commonv1 "github.com/pqcota/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"github.com/pqcota/pqcota/pkg/kernel/posture"
	"google.golang.org/grpc"
)

// ── 픽스처 빌더 (RFC 8446/4253 와이어 포맷; 파서와 독립적인 ground truth) ──

func u16b(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }

func vec16(data []byte) []byte { return append(u16b(uint16(len(data))), data...) }
func vec8(data []byte) []byte  { return append([]byte{byte(len(data))}, data...) }

func tlsRecord(handshake []byte) []byte {
	return append([]byte{0x16, 0x03, 0x01}, vec16(handshake)...)
}

func handshakeMsg(msgType byte, body []byte) []byte {
	l := len(body)
	return append([]byte{msgType, byte(l >> 16), byte(l >> 8), byte(l)}, body...)
}

func tlsExt(etype uint16, data []byte) []byte {
	return append(u16b(etype), vec16(data)...)
}

// clientHello — supported_groups [x25519, X25519MLKEM768] 담은 ClientHello 레코드.
func clientHelloMLKEM() []byte {
	groups := append(u16b(0x001d), u16b(0x11ec)...) // x25519, X25519MLKEM768
	sg := tlsExt(0x000a, vec16(groups))             // supported_groups: list_len(2)+groups
	sv := tlsExt(0x002b, vec8(u16b(0x0304)))        // supported_versions: list_len(1)+TLS1.3
	exts := append(sg, sv...)
	body := []byte{0x03, 0x03}                  // client_version
	body = append(body, make([]byte, 32)...)    // random
	body = append(body, vec8(nil)...)           // session_id (empty)
	body = append(body, vec16(u16b(0x1301))...) // cipher_suites: TLS_AES_128_GCM_SHA256
	body = append(body, vec8([]byte{0x00})...)  // compression_methods: null
	body = append(body, vec16(exts)...)         // extensions
	return tlsRecord(handshakeMsg(1, body))
}

// serverHelloX25519MLKEM — key_share=X25519MLKEM768 선택한 ServerHello 레코드.
func serverHelloX25519MLKEM() []byte {
	share := append(u16b(0x11ec), vec16(make([]byte, 32))...) // group + key_exchange(len2+key)
	ks := tlsExt(0x0033, share)                               // key_share (server_share)
	sv := tlsExt(0x002b, u16b(0x0304))                        // supported_versions: selected TLS1.3
	exts := append(sv, ks...)
	body := []byte{0x03, 0x03}
	body = append(body, make([]byte, 32)...)
	body = append(body, vec8(nil)...)
	body = append(body, u16b(0x1301)...) // cipher_suite
	body = append(body, 0x00)            // compression_method
	body = append(body, vec16(exts)...)
	return tlsRecord(handshakeMsg(2, body))
}

// sshKexInitSntrup — sntrup761x25519 최선호로 담은 SSH KEXINIT 패킷.
func sshKexInitSntrup() []byte {
	kex := "sntrup761x25519-sha512,curve25519-sha256,ecdh-sha2-nistp256"
	// SSH_MSG_KEXINIT = 20 (RFC 4253 §12). 구현 상수를 빌려 쓰면 그 값이 틀려도 통과한다.
	payload := []byte{20}
	payload = append(payload, make([]byte, 16)...) // cookie
	nl := make([]byte, 4)
	binary.BigEndian.PutUint32(nl, uint32(len(kex)))
	payload = append(payload, nl...)
	payload = append(payload, []byte(kex)...)
	padLen := byte(0)
	pkt := make([]byte, 4)
	binary.BigEndian.PutUint32(pkt, uint32(len(payload)+1)) // packet_length = payload + padding_length byte
	pkt = append(pkt, padLen)
	pkt = append(pkt, payload...)
	return pkt
}

// ── TD-NETWORK-1: ClientHello supported_groups 추출 ──
func TestParseClientHello(t *testing.T) {
	hs, err := network.ParseTLSHandshake(clientHelloMLKEM())
	if err != nil {
		t.Fatal(err)
	}
	if hs.Protocol != "TLS" || hs.Role != "client" {
		t.Errorf("protocol/role = %s/%s", hs.Protocol, hs.Role)
	}
	if hs.Version != "TLS1.3" {
		t.Errorf("version = %s, want TLS1.3", hs.Version)
	}
	want := map[string]bool{"x25519": false, "X25519MLKEM768": false}
	for _, g := range hs.OfferedGroups {
		if _, ok := want[g]; ok {
			want[g] = true
		}
	}
	for g, seen := range want {
		if !seen {
			t.Errorf("supported_groups에 %s 없음: %v", g, hs.OfferedGroups)
		}
	}
}

// ── TD-NETWORK-2: ServerHello 선택 그룹·cipher·version → negotiated_group + 등급 연동 ──
func TestParseServerHello(t *testing.T) {
	hs, err := network.ParseTLSHandshake(serverHelloX25519MLKEM())
	if err != nil {
		t.Fatal(err)
	}
	if hs.Role != "server" {
		t.Errorf("role = %s, want server", hs.Role)
	}
	if hs.NegotiatedGroup != "X25519MLKEM768" {
		t.Errorf("negotiated_group = %q, want X25519MLKEM768", hs.NegotiatedGroup)
	}
	if hs.Cipher != "TLS_AES_128_GCM_SHA256" {
		t.Errorf("cipher = %q", hs.Cipher)
	}
	if hs.Version != "TLS1.3" {
		t.Errorf("version = %q", hs.Version)
	}
	// 코어 등급 파생과 맞물리는지 교차 확인.
	if posture.Classify(hs.NegotiatedGroup, hs.Cipher) != discoveryv1.QuantumPosture_QUANTUM_POSTURE_PQC_HYBRID {
		t.Error("X25519MLKEM768 → PQC_HYBRID(🟢)이어야")
	}
}

// ── TD-NETWORK-3: SSH KEXINIT sntrup761x25519 관측 ──
func TestParseSSHKexInit(t *testing.T) {
	hs, err := network.ParseSSHKexInit(sshKexInitSntrup())
	if err != nil {
		t.Fatal(err)
	}
	if hs.Protocol != "SSH" {
		t.Errorf("protocol = %s", hs.Protocol)
	}
	found := false
	for _, k := range hs.OfferedGroups {
		if k == "sntrup761x25519-sha512" {
			found = true
		}
	}
	if !found {
		t.Errorf("kex 목록에 sntrup761x25519-sha512 없음: %v", hs.OfferedGroups)
	}
	// ★ KEXINIT 하나는 **제안**일 뿐이다 — 협상 결과로 채우면 안 된다(§2.1·§2.5).
	// (이전엔 "최선호 → 🟢"를 단언해 버그를 정답으로 못박고 있었다. 그 단언을 뒤집는다.)
	if hs.NegotiatedGroup != "" {
		t.Errorf("단일 KEXINIT에서 negotiated를 채우면 안 된다(제안≠협상): %q", hs.NegotiatedGroup)
	}
}

// 클라이언트가 sntrup761을 제안해도 **서버가 지원 안 하면 협상은 고전**이다.
// 실측에서 레거시 서버(OpenSSH 8.2, PQC KEX 없음)와의 SSH가 🟢 PQC로 잘못 보고됐다.
func TestNegotiateSSHKex(t *testing.T) {
	modern := []string{"sntrup761x25519-sha512@openssh.com", "curve25519-sha256", "ecdh-sha2-nistp256"}
	legacy := []string{"curve25519-sha256", "ecdh-sha2-nistp256", "diffie-hellman-group14-sha256"}

	// 레거시 서버 → 공통은 고전. 여기서 sntrup을 내면 "실제 협상"을 거짓 보고하는 것이다.
	if got := network.NegotiateSSHKex(modern, legacy); got != "curve25519-sha256" {
		t.Errorf("레거시 서버와의 협상 = %q, want curve25519-sha256 (🔴)", got)
	}
	if posture.Classify(network.NegotiateSSHKex(modern, legacy), "") == discoveryv1.QuantumPosture_QUANTUM_POSTURE_PQC_HYBRID {
		t.Error("서버가 PQC KEX를 지원 안 하는데 🟢로 등급되면 안 된다")
	}
	// 양쪽 다 현대 → 클라이언트 선호대로 PQC.
	if got := network.NegotiateSSHKex(modern, modern); got != "sntrup761x25519-sha512@openssh.com" {
		t.Errorf("현대↔현대 협상 = %q, want sntrup761", got)
	}
	// 선호 순서는 **클라이언트**가 정한다(RFC 4253 §7.1) — 서버 순서가 반대여도 결과는 같다.
	rev := []string{"ecdh-sha2-nistp256", "curve25519-sha256", "sntrup761x25519-sha512@openssh.com"}
	if got := network.NegotiateSSHKex(modern, rev); got != "sntrup761x25519-sha512@openssh.com" {
		t.Errorf("클라이언트 선호가 결정해야: %q", got)
	}
	// 한쪽만 관측 → 미상. 본 것만으로 협상을 지어내지 않는다(§2.5).
	for name, got := range map[string]string{
		"서버 미관측":    network.NegotiateSSHKex(modern, nil),
		"클라이언트 미관측": network.NegotiateSSHKex(nil, modern),
		"공통 없음":     network.NegotiateSSHKex([]string{"a"}, []string{"b"}),
	} {
		if got != "" {
			t.Errorf("%s: 협상 미상이어야 하는데 %q", name, got)
		}
	}
	// 미상은 ⚪로 분류되어야 한다(🟢/🔴 어느 쪽으로도 단정하지 않는다).
	if p := posture.Classify("", ""); p == discoveryv1.QuantumPosture_QUANTUM_POSTURE_PQC_HYBRID ||
		p == discoveryv1.QuantumPosture_QUANTUM_POSTURE_CLASSICAL {
		t.Errorf("협상 미상은 불명이어야: %v", p)
	}
}

// ── TD-NETWORK-4: 파싱 → ObservedEdge 빌드 ──
func TestBuildEdge(t *testing.T) {
	hs, _ := network.ParseTLSHandshake(serverHelloX25519MLKEM())
	conn := network.ConnTuple{SrcNode: "web-01", DstAddr: "10.0.1.20:8443", Port: 8443}
	e := network.BuildEdge(conn, hs)
	if e.GetSrcNodeId() != "web-01" || e.GetPort() != 8443 {
		t.Errorf("tuple 매핑 실패: %+v", e)
	}
	if e.GetProtocol() != discoveryv1.NetworkProtocol_NETWORK_PROTOCOL_TLS {
		t.Errorf("protocol = %v", e.GetProtocol())
	}
	if e.GetRole() != discoveryv1.EdgeRole_EDGE_ROLE_SERVER {
		t.Errorf("role = %v, want SERVER(ServerHello)", e.GetRole())
	}
	if e.GetNegotiatedGroup() != "X25519MLKEM768" {
		t.Errorf("negotiated_group = %q", e.GetNegotiatedGroup())
	}
	if e.GetDetectionMethod() != commonv1.DetectionMethod_DETECTION_METHOD_RUNTIME_INTROSPECTION {
		t.Errorf("detection_method = %v", e.GetDetectionMethod())
	}
}

// ── TD-NETWORK-5: 암호화된 핸드셰이크(QUIC) → 그룹 불명, 고전 단정 금지 ──
func TestQUICUnknownPosture(t *testing.T) {
	hs := &network.Handshake{Protocol: "QUIC", NegotiatedGroup: ""} // 협상 파라미터 관측하지 못함
	e := network.BuildEdge(network.ConnTuple{SrcNode: "web", DstAddr: "203.0.113.5:443"}, hs)
	if e.GetNegotiatedGroup() != "" {
		t.Error("QUIC는 협상 그룹 불명이어야(빈 문자열)")
	}
	if posture.Classify(e.GetNegotiatedGroup(), "") != discoveryv1.QuantumPosture_QUANTUM_POSTURE_UNSPECIFIED {
		t.Error("불명은 ⚪ UNSPECIFIED — 고전으로 단정 금지(라이선스 정리)")
	}
}

// ── TD-NETWORK-6: BuildResult 관측 레인 — NETWORK 커버, crypto_runtime 미상(CBOM 없음) ──
func TestBuildResult(t *testing.T) {
	e := network.BuildEdge(network.ConnTuple{SrcNode: "web", DstAddr: "a:443"}, &network.Handshake{Protocol: "TLS", NegotiatedGroup: "x25519"})
	res := network.BuildResult("web", []*discoveryv1.ObservedEdge{e}, "")
	if len(res.GetObservedEdges()) != 1 {
		t.Fatalf("observed_edges = %d", len(res.GetObservedEdges()))
	}
	if len(res.GetCbomCyclonedx()) != 0 {
		t.Error("네트워크 엣지는 노드 내부 CBOM을 채우지 않는다(미상)")
	}
	covered := res.GetCompleteness().GetLayersCovered()
	if len(covered) != 1 || covered[0] != commonv1.CollectionLayer_COLLECTION_LAYER_NETWORK {
		t.Errorf("layers_covered = %v, want [NETWORK]", covered)
	}
}

// ── TD-NETWORK-7: 관측 구간 갭 note (미관측 ≠ 부재) ──
func TestBuildResult_windowNote(t *testing.T) {
	res := network.BuildResult("web", nil, "")
	if res.GetCompleteness().GetNote() == "" {
		t.Error("관측 구간 한계를 completeness note로 정직히 표기해야(TD-NETWORK-7)")
	}
}

// ── TD-NETWORK-8: 자기참조 회피 ──
func TestShouldObserve_selfReference(t *testing.T) {
	self := map[string]bool{"web-01": true, "10.0.1.10:443": true}
	if network.ShouldObserve(network.ConnTuple{SrcNode: "web-01", DstNodeID: "web-01"}, self) {
		t.Error("자기 노드 대상은 제외해야")
	}
	if network.ShouldObserve(network.ConnTuple{SrcNode: "web-01", DstAddr: "10.0.1.10:443"}, self) {
		t.Error("자기 주소 대상은 제외해야")
	}
	if !network.ShouldObserve(network.ConnTuple{SrcNode: "web-01", DstAddr: "10.0.1.20:443"}, self) {
		t.Error("정상 상대는 관측해야")
	}
}

// ── TD-NETWORK-9: off-scope dst = 원시 주소(코어 등재판정 입력) ──
func TestBuildEdge_offScopeRawAddr(t *testing.T) {
	e := network.BuildEdge(network.ConnTuple{SrcNode: "web", DstNodeID: "", DstAddr: "203.0.113.5:443", Port: 443},
		&network.Handshake{Protocol: "TLS", NegotiatedGroup: "x25519"})
	if e.GetDstNodeId() != "" {
		t.Error("미해소 상대는 dst_node_id 비워야(코어가 등재 판정)")
	}
	if e.GetDstAddr() != "203.0.113.5:443" {
		t.Errorf("dst_addr = %q", e.GetDstAddr())
	}
}

// ── TD-NETWORK-10: Describe 능력 신고 ──
func TestDescribe(t *testing.T) {
	caps, _ := network.NewService(nil, nil).Describe(context.Background(), &discoveryv1.DescribeRequest{})
	if caps.GetCollectorId() != "network-collector" {
		t.Errorf("collector_id = %q", caps.GetCollectorId())
	}
	if caps.GetInvasive() {
		t.Error("수동·비침습이어야(invasive=false)")
	}
	if len(caps.GetLayers()) != 1 || caps.GetLayers()[0] != commonv1.CollectionLayer_COLLECTION_LAYER_NETWORK {
		t.Errorf("layers = %v, want [NETWORK]", caps.GetLayers())
	}
	if len(caps.GetCryptoRuntimes()) != 0 {
		t.Error("네트워크 엣지는 런타임 미상 → crypto_runtimes 비어야")
	}
}

// ── Collect: self-reference 필터 + 노드별 스트림 ──
type sliceSource struct{ obs []network.Observation }

func (s sliceSource) Observe([]string, map[string]string) ([]network.Observation, error) {
	return s.obs, nil
}

type fakeStream struct {
	grpc.ServerStreamingServer[discoveryv1.CollectionResult]
	sent []*discoveryv1.CollectionResult
}

func (f *fakeStream) Send(r *discoveryv1.CollectionResult) error {
	f.sent = append(f.sent, r)
	return nil
}

func TestCollect_filtersSelf(t *testing.T) {
	hs, _ := network.ParseTLSHandshake(serverHelloX25519MLKEM())
	src := sliceSource{obs: []network.Observation{
		{Conn: network.ConnTuple{SrcNode: "web", DstAddr: "10.0.1.20:8443"}, HS: hs},
		{Conn: network.ConnTuple{SrcNode: "web", DstAddr: "127.0.0.1:8443"}, HS: hs}, // 자기참조
	}}
	svc := network.NewService(src, map[string]bool{"127.0.0.1:8443": true})
	fs := &fakeStream{}
	if err := svc.Collect(&discoveryv1.CollectRequest{TargetNodeIds: []string{"web"}}, fs); err != nil {
		t.Fatal(err)
	}
	if len(fs.sent) != 1 {
		t.Fatalf("결과 %d개, want 1(노드 web)", len(fs.sent))
	}
	if n := len(fs.sent[0].GetObservedEdges()); n != 1 {
		t.Errorf("엣지 %d개, want 1(자기참조 제외)", n)
	}
}

// ── 구간이 중단됐을 때: 중단과 무관측을 같은 얼굴로 내보내지 않는다 ──
type truncSource struct {
	sliceSource
	cause error
}

func (t truncSource) WindowTruncated() (bool, error) { return true, t.cause }

// TD-NETWORK-19 — 구간이 읽기 오류로 중단되면 결과가 구간 전체를 대표하지 않는다. 엣지가 하나도 없을
// 때가 특히 위험하다 — 아무 말도 안 하면 "핸드셰이크 없음"으로 읽혀 결함이 갭으로
// 위장된다(§2.6). 그래서 엣지가 없어도 노드별 결과를 내고 완전성 노트에 중단을 적는다.
func TestCollect_marksTruncatedWindow(t *testing.T) {
	const want = "관측 구간이 중단됐다"
	cause := errors.New("recvfrom: input/output error")

	// ① 엣지가 하나도 없는데 구간이 중단된 경우.
	svc := network.NewService(truncSource{cause: cause}, nil)
	fs := &fakeStream{}
	if err := svc.Collect(&discoveryv1.CollectRequest{TargetNodeIds: []string{"web"}}, fs); err != nil {
		t.Fatal(err)
	}
	if len(fs.sent) != 1 {
		t.Fatalf("결과 %d개 — 엣지가 없어도 중단 사실은 나가야 한다", len(fs.sent))
	}
	note := fs.sent[0].GetCompleteness().GetNote()
	if !strings.Contains(note, want) || !strings.Contains(note, cause.Error()) {
		t.Errorf("중단과 사유가 완전성 노트에 없다: %q", note)
	}

	// ② 엣지가 있어도 그 결과 역시 구간 전체를 대표하지 않는다.
	hs, _ := network.ParseTLSHandshake(serverHelloX25519MLKEM())
	src := truncSource{sliceSource: sliceSource{obs: []network.Observation{
		{Conn: network.ConnTuple{SrcNode: "web", DstAddr: "10.0.1.20:8443"}, HS: hs}}}}
	fs2 := &fakeStream{}
	if err := network.NewService(src, nil).Collect(
		&discoveryv1.CollectRequest{TargetNodeIds: []string{"web"}}, fs2); err != nil {
		t.Fatal(err)
	}
	if len(fs2.sent) != 1 || !strings.Contains(fs2.sent[0].GetCompleteness().GetNote(), want) {
		t.Errorf("엣지가 있을 때도 중단을 표시해야: %+v", fs2.sent)
	}

	// ③ 중단되지 않은 소스는 이 문구가 뜨면 안 된다 — 매번 뜨는 경고는 읽히지 않는다.
	fs3 := &fakeStream{}
	if err := network.NewService(src.sliceSource, nil).Collect(
		&discoveryv1.CollectRequest{TargetNodeIds: []string{"web"}}, fs3); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fs3.sent[0].GetCompleteness().GetNote(), want) {
		t.Error("중단되지 않았는데 중단 노트가 떴다")
	}
}
