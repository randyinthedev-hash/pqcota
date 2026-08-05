package openssl

import (
	"strings"
	"testing"
)

// mergeByPath — 서로 다른 앱이 같은 .so를 로드하면 app_key를 합집합(#3 host-wide 다중 귀속).
func TestMergeByPathUnionsAppKeys(t *testing.T) {
	// payment.service와 api.service가 같은 libcrypto.so.3을, batch.service는 libssl.so.3만.
	perPID := [][]Detection{
		{{Lib: "libcrypto", Path: "/usr/lib/libcrypto.so.3", AppKeys: []string{"payment.service"}}},
		{{Lib: "libcrypto", Path: "/usr/lib/libcrypto.so.3", AppKeys: []string{"api.service"}}},
		{{Lib: "libssl", Path: "/usr/lib/libssl.so.3", AppKeys: []string{"batch.service"}}},
		{{Lib: "libcrypto", Path: "/usr/lib/libcrypto.so.3", AppKeys: []string{"payment.service"}}}, // 중복 → 제거
	}
	out := mergeByPath(perPID)

	if len(out) != 2 {
		t.Fatalf("경로별 유니크 %d개(2 기대): %v", len(out), out)
	}
	byPath := map[string][]string{}
	for _, d := range out {
		byPath[d.Path] = d.AppKeys
	}
	crypto := byPath["/usr/lib/libcrypto.so.3"]
	if strings.Join(crypto, ",") != "api.service,payment.service" { // 합집합·정렬·중복제거
		t.Errorf("공유 .so 다중 귀속 실패: %v", crypto)
	}
	if ssl := byPath["/usr/lib/libssl.so.3"]; len(ssl) != 1 || ssl[0] != "batch.service" {
		t.Errorf("단일 귀속: %v", ssl)
	}
}

// 빈 app_key(비-PID 스캔·귀속 불가)는 nil로 — CSV 속성이 빈 문자열이어도 무해.
func TestMergeByPathNoAppKeys(t *testing.T) {
	out := mergeByPath([][]Detection{{{Path: "/usr/lib/libcrypto.so.3"}}})
	if len(out) != 1 || out[0].AppKeys != nil {
		t.Errorf("귀속 없으면 nil: %v", out)
	}
}
