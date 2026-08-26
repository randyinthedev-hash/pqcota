package main

import "example.test/provisioning"

// 이름을 Test로 시작하지 않는다 — 테스트 함수 수를 세는 게이트가 fixture를 진짜로 세면 안 된다.
func usesTheGate() { _ = provisioning.Executable("x") }
