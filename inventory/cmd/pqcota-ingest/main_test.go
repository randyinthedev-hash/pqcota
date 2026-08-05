package main

import "testing"

// parseResultDocs는 단일 객체(compact·multiline)와 JSON Lines를 모두 감당해야 한다.
// jvm attach 경로가 노드당 JVM 여럿을 JSONL로 방출하므로 후자가 필수다.

const oneCompact = `{"envelope":{"targetNodeId":"n1"},"rawFormat":"x"}`

const oneMultiline = `{
  "envelope": {"targetNodeId": "n1"},
  "rawFormat": "x"
}`

// JSON Lines — 한 줄에 하나. 빈 줄·CBOM이 아닌 잡음 줄은 건너뛴다.
const jsonl = `{"envelope":{"targetNodeId":"jvm-a"},"rawFormat":"x"}

{"envelope":{"targetNodeId":"jvm-b"},"rawFormat":"x"}
`

func TestParseSingleCompact(t *testing.T) {
	got := parseResultDocs([]byte(oneCompact))
	if len(got) != 1 || got[0].GetEnvelope().GetTargetNodeId() != "n1" {
		t.Fatalf("단일 compact 파싱 실패: %+v", got)
	}
}

// ★ pretty-print된 단일 객체는 줄별로는 안 깨진다 — 단일 시도가 JSONL보다 먼저여야 한다.
func TestParseSingleMultiline(t *testing.T) {
	got := parseResultDocs([]byte(oneMultiline))
	if len(got) != 1 || got[0].GetEnvelope().GetTargetNodeId() != "n1" {
		t.Fatalf("단일 multiline 파싱 실패(줄별로 깨졌을 수 있음): %+v", got)
	}
}

func TestParseJSONL(t *testing.T) {
	got := parseResultDocs([]byte(jsonl))
	if len(got) != 2 {
		t.Fatalf("JSONL은 2건이어야 함(빈 줄 제외): %d", len(got))
	}
	if got[0].GetEnvelope().GetTargetNodeId() != "jvm-a" || got[1].GetEnvelope().GetTargetNodeId() != "jvm-b" {
		t.Errorf("JSONL 순서·내용 불일치: %+v", got)
	}
}

// 잡음 줄이 섞여도 유효한 CollectionResult만 모은다(다른 JSON 파일 오인 방지).
func TestParseJSONLSkipsGarbage(t *testing.T) {
	mixed := `{"envelope":{"targetNodeId":"ok"},"rawFormat":"x"}
{"not":"a collection result but still json"}
plain garbage
`
	got := parseResultDocs([]byte(mixed))
	// protojson은 알 수 없는 필드에 엄격해 2·3번째 줄은 파싱 실패로 건너뛴다.
	// 유효한 첫 줄은 반드시 포함되어야 한다.
	var sawOK bool
	for _, r := range got {
		if r.GetEnvelope().GetTargetNodeId() == "ok" {
			sawOK = true
		}
	}
	if !sawOK {
		t.Errorf("유효한 첫 줄이 유실됨: %+v", got)
	}
}
