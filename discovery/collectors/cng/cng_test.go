package cng

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
)

func init() { now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() } }

// TD-CNG-1 — 관측한 provider 순서를 그대로 싣는다.
// 순서가 곧 우선순위라(수용 원칙 §2.2), 정렬하거나 집합으로 만들면 "어느 provider가 먼저
// 서비스하나"라는 질문에 답할 수 없게 된다.
func TestProviderOrderIsPreserved(t *testing.T) {
	obs := Observation{Providers: []string{"Microsoft Primitive Provider", "Vendor KSP", "Microsoft Smart Card Key Storage Provider"}}
	res := BuildResult("win-01", obs, nil)

	got := providerSetProp(t, res.GetCbomCyclonedx())
	want := "Microsoft Primitive Provider,Vendor KSP,Microsoft Smart Card Key Storage Provider"
	if got != want {
		t.Errorf("provider_set이 관측 순서와 다르다:\n got %q\nwant %q", got, want)
	}
}

// TD-CNG-2 — 관측하지 못한 것과 "provider가 없다"를 같은 얼굴로 내보내지 않는다(§2.6).
func TestUnobservedIsNotAbsence(t *testing.T) {
	failed := BuildResult("win-01", Observation{}, ErrNotWindows)
	if len(failed.GetCompleteness().GetLayersCovered()) != 0 {
		t.Error("관측하지 못했는데 계층을 커버로 셌다 — 인벤토리에서 부재로 읽힌다")
	}
	if !strings.Contains(failed.GetCompleteness().GetNote(), "관측하지 못한 것이다") {
		t.Errorf("못 봤다는 사실이 노트에 없다: %q", failed.GetCompleteness().GetNote())
	}
	if len(failed.GetCbomCyclonedx()) != 0 || len(failed.GetRawCapture()) != 0 {
		t.Error("관측하지 못했는데 본문이 실렸다")
	}

	empty := BuildResult("win-01", Observation{}, nil)
	covered := empty.GetCompleteness().GetLayersCovered()
	if len(covered) != 1 || covered[0] != commonv1.CollectionLayer_COLLECTION_LAYER_CNG_INTROSPECTION {
		t.Errorf("봤는데 없었던 것은 커버다 — covered=%v", covered)
	}
}

// TD-CNG-3 — 원본이 없으면 형식 이름도 비운다(§1.2 — 재정규화할 것이 없는데 있다고 하지 않는다).
func TestRawFormatEmptyWithoutRaw(t *testing.T) {
	res := BuildResult("win-01", Observation{}, nil)
	if res.GetRawFormat() != "" {
		t.Errorf("원본이 없는데 형식 이름이 붙었다: %q", res.GetRawFormat())
	}
	full := BuildResult("win-01", Observation{Providers: []string{"P"}}, nil)
	if full.GetRawFormat() == "" || len(full.GetRawCapture()) == 0 {
		t.Error("관측했으면 원본과 형식 이름이 함께 있어야 한다")
	}
}

// TD-CNG-4 — 알고리즘은 원본에만 실린다. 계약(CngAxes)에 자리가 없는 것을 파생 뷰에 지어내지
// 않으면서도, 관측한 사실은 버리지 않는다.
func TestAlgorithmsRideOnRawOnly(t *testing.T) {
	obs := Observation{
		Providers:  []string{"Microsoft Primitive Provider"},
		Algorithms: []Algorithm{{Name: "ML-KEM", Class: "secret-agreement"}},
	}
	res := BuildResult("win-01", obs, nil)
	if strings.Contains(string(res.GetCbomCyclonedx()), "ML-KEM") {
		t.Error("계약에 자리 없는 축이 CycloneDX로 샜다")
	}
	var back Observation
	if err := json.Unmarshal(res.GetRawCapture(), &back); err != nil {
		t.Fatalf("원본이 다시 읽히지 않는다: %v", err)
	}
	if len(back.Algorithms) != 1 || back.Algorithms[0].Name != "ML-KEM" {
		t.Errorf("관측한 알고리즘이 원본에서 사라졌다: %+v", back.Algorithms)
	}
}

func providerSetProp(t *testing.T, cyclone []byte) string {
	t.Helper()
	var doc struct {
		Components []struct {
			Properties []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"properties"`
		} `json:"components"`
	}
	if err := json.Unmarshal(cyclone, &doc); err != nil {
		t.Fatalf("CycloneDX 파싱 실패: %v", err)
	}
	for _, c := range doc.Components {
		for _, p := range c.Properties {
			if p.Name == "pqcota:cng.provider_set" {
				return p.Value
			}
		}
	}
	t.Fatal("pqcota:cng.provider_set 속성이 없다")
	return ""
}

// TD-CNG-6 — dwClass 매핑을 실측에 못 박는다.
//
// 첫 실측(Windows 11 26200)에서 50개 중 18개가 빈 종류로 나오고 DH·ECDH가 `secret-agreement`가
// 아니라 `asymmetric-encryption`으로 **틀리게** 붙었다. 원인은 열거 **요청**의 연산 비트마스크와
// 반환값의 **인터페이스 상수**를 같은 어휘로 본 것이다. 값이 겹쳐 조용히 틀리는 자리라 못 박는다.
func TestAlgorithmClassFollowsTheInterfaceConstants(t *testing.T) {
	for _, c := range []struct {
		dwClass uint32
		want    string
		seenAs  string // 그 인터페이스로 실제 관측된 알고리즘(실측 근거)
	}{
		{1, "cipher", "AES"},
		{2, "hash", "SHA256"},
		{3, "asymmetric-encryption", "RSA"},
		{4, "secret-agreement", "ECDH_P256"},
		{5, "signature", "ML-DSA"},
		{6, "rng", "RNG"},
		{7, "key-derivation", "HKDF"},
	} {
		if got := AlgorithmClass(c.dwClass); got != c.want {
			t.Errorf("dwClass=%d(%s): got %q, want %q", c.dwClass, c.seenAs, got, c.want)
		}
	}
	// 모르는 값을 아는 것으로 적지 않는다(§2.5). 0과 미래 값 둘 다 빈 값이어야 한다.
	for _, unknown := range []uint32{0, 8, 0x10, 0x40, 99} {
		if got := AlgorithmClass(unknown); got != "" {
			t.Errorf("모르는 dwClass=%d에 종류를 붙였다: %q", unknown, got)
		}
	}
}
