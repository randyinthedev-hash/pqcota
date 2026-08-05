//go:build linux

package network

import (
	"errors"
	"testing"
	"time"
)

// TD-NETWORK-13: CAP_NET_RAW 없이 AF_PACKET 열기 → ErrCaptureUnavailable(크래시 아님).
// 권한이 있는 환경(root)에선 소켓이 열리므로 강등 경로 미검증 → 스킵.
func TestLiveSource_noCapPerm(t *testing.T) {
	ls := &LiveSource{Node: "web-01", Window: 100 * time.Millisecond}
	_, err := ls.Observe(nil, nil)
	if err == nil {
		t.Skip("CAP_NET_RAW 있음(소켓 열림) — 이 환경에선 강등 경로 미적용")
	}
	if !errors.Is(err, ErrCaptureUnavailable) {
		t.Fatalf("권한 없음은 ErrCaptureUnavailable로 감싸야: %v", err)
	}
}

// edgeFor: 로컬이 클라이언트인 엣지만 client→server(낮은 포트=서버) 방향으로 방출.
func TestEdgeFor_clientOnly(t *testing.T) {
	ls := &LiveSource{Node: "web-01", SelfIPs: map[string]bool{"10.0.1.10": true}}

	// 로컬(10.0.1.10)이 클라이언트(고포트)로 서버 8443에 연결 → 방출, dst=서버.
	out := &Segment{SrcIP: "10.0.1.10", DstIP: "10.0.1.20", SrcPort: 40000, DstPort: 8443}
	if c, ok := ls.edgeFor(out); !ok || c.DstAddr != "10.0.1.20:8443" || c.Port != 8443 || !c.SrcInitiated {
		t.Errorf("아웃바운드 클라이언트 엣지: ok=%v %+v", ok, c)
	}
	// 같은 연결의 반대 방향(서버→클라이언트 응답)도 서버=낮은포트로 판정 → 여전히 로컬=클라이언트 → 방출.
	in := &Segment{SrcIP: "10.0.1.20", DstIP: "10.0.1.10", SrcPort: 8443, DstPort: 40000}
	if c, ok := ls.edgeFor(in); !ok || c.DstAddr != "10.0.1.20:8443" {
		t.Errorf("역방향에서도 동일 엣지: ok=%v %+v", ok, c)
	}
	// 로컬이 서버(8443 리슨)이고 상대가 클라이언트 → 방출 안 함(상대가 보고).
	ls2 := &LiveSource{Node: "app-01", SelfIPs: map[string]bool{"10.0.1.20": true}}
	if _, ok := ls2.edgeFor(out); ok {
		t.Error("서버측은 방출하지 않아야(중복 방지)")
	}
}

// ★ 관측 창은 **끝까지 채워져야 한다.** 원시 syscall은 시그널에 깨어 EINTR을 돌려주는데(Go 런타임이
// 고루틴 선점을 위해 SIGURG를 보낸다), 그걸 치명적 오류로 보고 루프를 나가면 창이 무작위 시점에
// 조용히 끝나고 결과가 "핸드셰이크 없음"이 된다 — 결함이 갭처럼 보인다(§2.7).
// 실측으로 재현했던 증상: 25초 창이 0·0·14·25초에 끝남.
//
// CAP_NET_RAW가 없으면 소켓 자체가 안 열려 이 경로를 못 타므로 스킵한다.
func TestObserveFillsTheWholeWindow(t *testing.T) {
	const window = 1200 * time.Millisecond
	ls := &LiveSource{Node: "web-01", Window: window}
	start := time.Now()
	if _, err := ls.Observe(nil, nil); err != nil {
		t.Skip("CAP_NET_RAW 없음 — 캡처 루프 미실행")
	}
	elapsed := time.Since(start)
	// 200ms 수신 타임아웃으로 폴링하므로 창보다 한 틱 정도만 짧을 수 있다. 그보다 일찍 끝났다면
	// 루프가 오류로 빠져나온 것이다.
	if elapsed < window-300*time.Millisecond {
		t.Errorf("창(%v)을 다 채우지 못하고 %v에 끝났다 — EINTR 등을 치명적으로 다루면 관측이 조용히 잘린다", window, elapsed)
	}
	if ls.Truncated {
		t.Errorf("정상 종료인데 Truncated가 섰다: %v", ls.TruncErr)
	}
}
