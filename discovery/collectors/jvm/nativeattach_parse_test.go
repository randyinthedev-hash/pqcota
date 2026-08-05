package jvm

import (
	"strings"
	"testing"
)

// 프로토콜 요청 조립 — 인자 슬롯이 정확히 3개여야 대상이 응답한다(모자라면 읽다 멈춘다).
func TestLoadAgentRequest(t *testing.T) {
	req := loadAgentRequest("/opt/collector.jar", "/tmp/out.txt")
	parts := strings.Split(strings.TrimSuffix(req, "\x00"), "\x00")
	want := []string{"1", "load", "instrument", "false", "/opt/collector.jar=/tmp/out.txt"}
	if len(parts) != len(want) {
		t.Fatalf("필드 수 %d, want %d: %q", len(parts), len(want), parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Errorf("필드[%d] = %q, want %q", i, parts[i], want[i])
		}
	}
	if !strings.HasSuffix(req, "\x00") {
		t.Error("마지막 인자도 NUL로 끝나야 한다")
	}
	// agentArgs가 없으면 '=' 없이 jar만.
	if got := loadAgentRequest("/a.jar", ""); !strings.Contains(got, "\x00/a.jar\x00") {
		t.Errorf("빈 인자면 '='를 붙이지 않아야: %q", got)
	}
}

// 응답 파싱 — 첫 줄이 리턴 코드. 0만 성공이고, 침묵을 성공으로 치지 않는다(§2.6).
func TestParseAttachResponse(t *testing.T) {
	if err := parseAttachResponse("0\n"); err != nil {
		t.Errorf("코드 0은 성공이어야: %v", err)
	}
	if err := parseAttachResponse("0\nAgent loaded\n"); err != nil {
		t.Errorf("코드 0 + 메시지도 성공: %v", err)
	}
	err := parseAttachResponse("101\nagent load failed: no such file\n")
	if err == nil {
		t.Fatal("0이 아니면 실패여야")
	}
	if !strings.Contains(err.Error(), "101") || !strings.Contains(err.Error(), "no such file") {
		t.Errorf("코드와 사유를 함께 알려야: %v", err)
	}
	// 대상이 아무 말도 안 했으면 로드 여부를 모른다 — 성공으로 단정하지 않는다.
	if err := parseAttachResponse(""); err == nil {
		t.Error("빈 응답을 성공으로 치면 안 된다")
	}
	if err := parseAttachResponse("garbage\n"); err == nil {
		t.Error("리턴 코드가 아닌 응답은 실패로 다뤄야")
	}
}

// 컨테이너 대상 — 소켓·트리거 파일 이름은 **네임스페이스 내부 PID**를 쓴다. 호스트 PID로
// 찾으면 못 만난다. NSpid 줄의 마지막 값이 가장 안쪽 네임스페이스다.
func TestParseNSpid(t *testing.T) {
	container := "Name:\tjava\nPid:\t4242\nNSpid:\t4242\t7\nUid:\t0\t0\t0\t0\n"
	if got := parseNSpid(container, 4242); got != 7 {
		t.Errorf("컨테이너 대상 nspid = %d, want 7(가장 안쪽)", got)
	}
	// 같은 네임스페이스면 값이 하나뿐.
	if got := parseNSpid("NSpid:\t100\n", 100); got != 100 {
		t.Errorf("동일 NS = %d, want 100", got)
	}
	// NSpid 줄이 없으면(구 커널) 호스트 PID를 그대로 쓴다.
	if got := parseNSpid("Name:\tjava\nPid:\t55\n", 55); got != 55 {
		t.Errorf("NSpid 없으면 fallback이어야: %d", got)
	}
}
