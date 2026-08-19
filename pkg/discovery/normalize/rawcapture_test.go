package normalize_test

import (
	"strings"
	"testing"

	"github.com/randyinthedev-hash/pqcota/discovery/collectors/jvm"
	"github.com/randyinthedev-hash/pqcota/discovery/collectors/network"
	"github.com/randyinthedev-hash/pqcota/discovery/collectors/openssl"
	discoveryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/discovery/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/inventory/declaration"
)

// 불변식: **raw_format이 있으면 raw_capture도 있다.**
//
// 계약은 "강화 규칙이 좋아지면 원본에서 재정규화한다"고 적는다(§1.2·§2.4 step 1). 형식
// 이름만 있고 내용이 없으면 재정규화할 것이 없어 그 약속이 거짓이 된다. 빌더는 늘어나기
// 마련이고 원본을 채우는 것은 잊기 쉬우므로, 불변식으로 못 박는다.
func TestRawFormatImpliesRawCapture(t *testing.T) {
	cases := map[string]*discoveryv1.CollectionResult{
		"openssl": openssl.BuildResult("cmdb://n1", []openssl.Detection{
			{Lib: "libssl", Path: "/usr/lib/libssl.so.3", Fork: "openssl", Version: "3.5.4"},
		}),
		"jvm":     jvm.BuildResult("cmdb://n1", jvm.ParseProviders("1|SUN|21|sun.security.provider.Sun\n2|BC|1.85|org.bouncycastle.jce.provider.BouncyCastleProvider")),
		"network": network.BuildResult("cmdb://n1", nil, ""),
	}
	decl, err := declaration.ImportCSV(strings.NewReader("node_id,crypto_runtime,component\ncmdb://n1,openssl,libssl\n"))
	if err != nil || len(decl) != 1 {
		t.Fatalf("declaration import: %v (count %d)", err, len(decl))
	}
	cases["declaration"] = decl[0]

	for name, res := range cases {
		if res.GetRawFormat() != "" && len(res.GetRawCapture()) == 0 {
			t.Errorf("%s: raw_format=%q but raw_capture is empty — there is nothing to re-normalize",
				name, res.GetRawFormat())
		}
	}
}

// 같은 관측이면 같은 바이트 — raw_capture는 서명이 덮는 값이라(§2.6) 흔들리면 검증이 깨진다.
func TestRawCaptureDeterministic(t *testing.T) {
	dets := []openssl.Detection{
		{Lib: "libcrypto", Path: "/usr/lib/libcrypto.so.3", AppKeys: []string{"a", "b"}},
		{Lib: "libssl", Path: "/usr/lib/libssl.so.3"},
	}
	if a, b := string(openssl.RawCapture(dets)), string(openssl.RawCapture(dets)); a != b {
		t.Errorf("the same input gave different bytes:\n%s\n%s", a, b)
	}
}
