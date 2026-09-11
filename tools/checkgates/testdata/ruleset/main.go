package fixture

// 검사기 fixture. 실제 적재 명령이 그랬듯 자기 문자열을 넘긴다.
func ingest() {
	_ = ingestCBOM("cbom-20260101", "ruleset-demo")
	_ = ingestCBOM("snap", "pqcota-enrich/v1") // 상수 값을 그대로 베낀 자리
}

// 이름표는 값이 아니다 — 잡으면 안 된다.
func render() string { return "node %s (snapshot %s · ruleset %s)" }

func ingestCBOM(prefix, ruleset string) string { return prefix + ruleset }
