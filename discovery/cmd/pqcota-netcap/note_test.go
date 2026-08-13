//go:build linux

package main

import "testing"

// TestAttributionNoteSaysWhatItDoesNotMean — 어느 앱인지 못 밝힌 것을 "앱 없음"으로 읽히게 두지 않는다.
//
// 빈 `app_key`만 남기고 끝내면 그 엣지는 "이 통신에 앱이 없다"로 읽힌다. 관측 갭을 부재로
// 적지 않는 것과 같은 규칙이 여기에도 적용된다(§2.6).
func TestAttributionNoteSaysWhatItDoesNotMean(t *testing.T) {
	if got := attributionNote(5, nil); got != "" {
		t.Errorf("전부 잡았는데 노트가 남았다: %q", got)
	}

	got := attributionNote(5, map[string]int{"소켓이 닫혔다": 2, "권한이 없다": 1})
	if got == "" {
		t.Fatal("못 잡은 것이 있는데 노트가 비었다")
	}
	for _, want := range []string{"5개 중 3개", "앱이 없다는 뜻이 아니다", "소켓이 닫혔다(2)", "권한이 없다(1)"} {
		if !contains(got, want) {
			t.Errorf("노트에 %q가 없다: %s", want, got)
		}
	}

	// 사유 순서가 흔들리면 같은 관측이 다른 스냅샷으로 보인다 — 내용 지문이 달라지기 때문.
	a := attributionNote(3, map[string]int{"가": 1, "나": 1, "다": 1})
	for i := 0; i < 5; i++ {
		if b := attributionNote(3, map[string]int{"다": 1, "가": 1, "나": 1}); b != a {
			t.Fatalf("사유 순서가 흔들린다:\n  %s\n  %s", a, b)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
