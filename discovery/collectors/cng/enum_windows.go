//go:build windows

package cng

import (
	"fmt"
	"sort"
	"unsafe"

	"golang.org/x/sys/windows"
)

// bcrypt.dll — CNG의 열거 API. 외부 도구(`certutil`·PowerShell)를 부르지 않고 직접 호출한다:
// 최소 발자국이고, 도구가 없거나 정책으로 막힌 서버에서도 관측 실패가 환경 탓으로 흩어지지
// 않는다(§2.3, openssl collector가 `ldd` 없이 ELF를 직접 읽는 것과 같은 원칙).
var (
	bcrypt = windows.NewLazySystemDLL("bcrypt.dll")

	procEnumRegisteredProviders = bcrypt.NewProc("BCryptEnumRegisteredProviders")
	procEnumAlgorithms          = bcrypt.NewProc("BCryptEnumAlgorithms")
	procFreeBuffer              = bcrypt.NewProc("BCryptFreeBuffer")
)

// cryptProviders — CRYPT_PROVIDERS(bcrypt.h). 이름 배열의 **순서가 곧 등록 순서**다.
type cryptProviders struct {
	count     uint32
	providers **uint16
}

// algorithmIdentifier — BCRYPT_ALGORITHM_IDENTIFIER(bcrypt.h).
type algorithmIdentifier struct {
	name  *uint16
	class uint32
	flags uint32
}

// 열거를 **요청**할 때 쓰는 연산 비트마스크(bcrypt.h). 반환값의 dwClass와는 다른 어휘다 —
// 그것은 인터페이스 상수이고 [AlgorithmClass]가 옮긴다. 둘을 같은 것으로 본 것이 첫 실측에서
// 잡힌 결함이다(값이 겹쳐 절반은 빈 값, DH·ECDH는 틀린 종류로 붙었다).
const (
	opCipher                    = 0x00000001
	opHash                      = 0x00000002
	opAsymmetricEncrypt         = 0x00000004
	opSecretAgreement           = 0x00000008
	opSignature                 = 0x00000010
	opRNG                       = 0x00000020
	opKeyDerivation             = 0x00000040
	opAll                       = opCipher | opHash | opAsymmetricEncrypt | opSecretAgreement | opSignature | opRNG | opKeyDerivation
	statusSuccess       uintptr = 0
)

// Enumerate — 이 머신에 등록된 CNG provider와 알고리즘을 관측한다.
//
// provider 열거가 실패하면 **에러로 돌려준다** — 빈 목록으로 내려가면 부재로 읽힌다(§2.6).
// 알고리즘 열거는 보조 축이라 실패해도 provider 관측까지는 살린다(부분 관측은 성공이다).
func Enumerate() (Observation, error) {
	providers, err := registeredProviders()
	if err != nil {
		return Observation{}, err
	}
	obs := Observation{Providers: providers}
	if algs, err := algorithms(); err == nil {
		obs.Algorithms = algs
	}
	return obs, nil
}

func registeredProviders() ([]string, error) {
	var size uint32
	var buf *cryptProviders
	st, _, _ := procEnumRegisteredProviders.Call(
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&buf)))
	if st != statusSuccess {
		return nil, ntStatus("BCryptEnumRegisteredProviders", st)
	}
	defer procFreeBuffer.Call(uintptr(unsafe.Pointer(buf)))
	if buf == nil || buf.count == 0 {
		return nil, nil
	}
	// 순서를 그대로 옮긴다 — 정렬하지 않는다. 우선순위가 목록 순서에 담겨 있다.
	names := unsafe.Slice(buf.providers, buf.count)
	out := make([]string, 0, buf.count)
	for _, p := range names {
		out = append(out, windows.UTF16PtrToString(p))
	}
	return out, nil
}

func algorithms() ([]Algorithm, error) {
	var count uint32
	var list *algorithmIdentifier
	st, _, _ := procEnumAlgorithms.Call(
		uintptr(opAll), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&list)), 0)
	if st != statusSuccess {
		return nil, ntStatus("BCryptEnumAlgorithms", st)
	}
	defer procFreeBuffer.Call(uintptr(unsafe.Pointer(list)))
	if list == nil || count == 0 {
		return nil, nil
	}
	items := unsafe.Slice(list, count)
	out := make([]Algorithm, 0, count)
	for _, a := range items {
		out = append(out, Algorithm{Name: windows.UTF16PtrToString(a.name), Class: AlgorithmClass(a.class)})
	}
	// 알고리즘은 우선순위가 아니라 집합이다. 열거 순서가 실행마다 흔들리면 같은 관측이 다른
	// 내용 지문이 되어 변화가 없는데 스냅샷이 늘어나므로 정렬해 결정론을 준다(§1.2).
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Class < out[j].Class
	})
	return out, nil
}

// ntStatus — NTSTATUS를 읽을 수 있는 오류로. bcrypt는 Win32 오류코드가 아니라 NTSTATUS를
// 돌려주므로 `windows.Errno`로 감싸면 엉뚱한 문구가 붙는다 — 코드를 그대로 적는다.
func ntStatus(call string, st uintptr) error {
	return fmt.Errorf("%s가 NTSTATUS 0x%08X를 돌려줬다", call, uint32(st))
}
