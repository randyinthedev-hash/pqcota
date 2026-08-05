//go:build !linux

// 비-Linux 스텁 — 네트워크 관측은 AF_PACKET 원시 소켓이라 리눅스에서만 성립한다.
// 이 파일이 없으면 macOS·Windows에서 `go build ./...`가 "build constraints exclude all Go files"로
// 깨진다. 개발은 다른 OS에서도 하므로, 빌드는 되게 하되 **실행하면 정직히 거부**한다.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "pqcota-netcap은 리눅스 전용이다(AF_PACKET 원시 소켓, CAP_NET_RAW).")
	fmt.Fprintln(os.Stderr, "관측 대상에 올릴 바이너리는 교차 빌드로 만든다:")
	fmt.Fprintln(os.Stderr, "  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./discovery/cmd/pqcota-netcap")
	os.Exit(2)
}
