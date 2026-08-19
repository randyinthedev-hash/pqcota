//go:build windows

package machineid

import "golang.org/x/sys/windows/registry"

// MachineGuidPath — Windows의 설치 단위 안정 식별자가 사는 곳. `/etc/machine-id`의 대응물이다.
// OS를 다시 깔면 바뀌고, 호스트명을 바꿔도 그대로다 — 노드 이력이 이름에 흔들리지 않게 하는 앵커.
const MachineGuidPath = `SOFTWARE\Microsoft\Cryptography`

// platformIDs — Windows 경로의 안정 지문.
//
// `certutil`·WMI·PowerShell을 부르지 않고 레지스트리를 직접 읽는다(§2.3, collector와 같은 원칙).
// **hardware_uuid는 아직 비운다** — SMBIOS UUID는 레지스트리 한 줄이 아니라 펌웨어 테이블을 떠야
// 나온다. 못 읽은 것을 지어내지 않고 빈 값으로 두면, self-id는 machine-id로 떨어져 이름에
// 흔들리지 않는다(§2.5 · §1.4).
func platformIDs() (machineID, hardwareUUID string) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, MachineGuidPath, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", ""
	}
	defer k.Close()
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", ""
	}
	return guid, ""
}
