// Command pqc-echo — 데모용 PQC TLS 트래픽 생성기(🟢 엣지).
// Go의 crypto/tls는 X25519MLKEM768 하이브리드를 협상하므로, 이 서버↔클라이언트 핸드셰이크를
// network-collector가 관측하면 실제 PQC 등급 엣지가 토폴로지에 뜬다.
// usage:
//
//	pqc-echo server <listen-addr>            # 예: :8443
//	pqc-echo client <server-addr> [count]    # 예: node-app:8443 5
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: pqc-echo server <addr> | pqc-echo client <addr> [count]")
		os.Exit(2)
	}
	mode, addr := os.Args[1], os.Args[2]
	curves := []tls.CurveID{tls.X25519MLKEM768} // PQC 하이브리드 강제

	switch mode {
	case "server":
		runServer(addr, curves)
	case "client":
		count := 5
		if len(os.Args) > 3 {
			if n, err := strconv.Atoi(os.Args[3]); err == nil {
				count = n
			}
		}
		runClient(addr, curves, count)
	default:
		fmt.Fprintln(os.Stderr, "unknown mode:", mode)
		os.Exit(2)
	}
}

func runServer(addr string, curves []tls.CurveID) {
	cfg := &tls.Config{
		Certificates:     []tls.Certificate{selfSigned()},
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: curves,
	}
	ln, err := tls.Listen("tcp", addr, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[pqc-echo] server %s (X25519MLKEM768)\n", addr)
	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}
		go func(c *tls.Conn) {
			defer c.Close()
			_ = c.Handshake()
			buf := make([]byte, 256)
			n, _ := c.Read(buf)
			c.Write(buf[:n])
		}(c.(*tls.Conn))
	}
}

func runClient(addr string, curves []tls.CurveID, count int) {
	cfg := &tls.Config{
		InsecureSkipVerify: true, // 데모: 자체서명 신뢰
		MinVersion:         tls.VersionTLS13,
		CurvePreferences:   curves,
	}
	ok := 0
	for i := 0; i < count; i++ {
		c, err := tls.Dial("tcp", addr, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[pqc-echo] dial %s: %v\n", addr, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		c.Write([]byte("pqcota-demo\n"))
		buf := make([]byte, 64)
		c.Read(buf)
		grp := c.ConnectionState().CurveID // 협상된 그룹
		c.Close()
		ok++
		fmt.Fprintf(os.Stderr, "[pqc-echo] %s handshake OK (curve=%v)\n", addr, grp)
		time.Sleep(300 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "[pqc-echo] %d/%d succeeded\n", ok, count)
}

func selfSigned() tls.Certificate {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "pqc-echo"},
		NotBefore:    time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	der, _ := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
