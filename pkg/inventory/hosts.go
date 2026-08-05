package inventory

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	inventoryv1 "github.com/pqcota/pqcota/gen/pqcota/inventory/v1"
)

// Host — 사용자 관리 hosts 파일의 한 행. **연결 비밀**(SSHUser/SSHKey/SSHPass)은 **런타임 전용**이며
// pqcota 인벤토리에 적재하지 않는다. 적재 대상은 Endpoint()(node_id·name·ip·port)뿐이다.
type Host struct {
	NodeID  string
	Name    string
	IP      string
	Port    int
	SSHUser string
	SSHKey  string // SSH 키 파일 경로 — 비밀, 미영속
	SSHPass string // 암호(권장 안 함) — 비밀, 미영속
}

// ParseHosts — 사용자 관리 CSV hosts 파일을 읽는다. 헤더 필수(node_id 필수, 나머지 선택·순서 자유).
// 예 헤더: node_id,name,ip,port,ssh_user,ssh_key
// discovery 실행 시마다 사용자가 이 파일을 지정 → pqcota가 읽어 Ansible로 처리한다.
func ParseHosts(r io.Reader) ([]Host, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // 행별 열 수 가변 허용
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("빈 파일")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(strings.ToLower(h))] = i
	}
	if _, ok := col["node_id"]; !ok {
		return nil, fmt.Errorf("헤더에 node_id 필요")
	}
	get := func(row []string, name string) string {
		if i, ok := col[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	var out []Host
	for _, row := range rows[1:] {
		if len(row) == 0 || strings.HasPrefix(strings.TrimSpace(row[0]), "#") {
			continue
		}
		nid := get(row, "node_id")
		if nid == "" {
			continue
		}
		port := 22
		if p := get(row, "port"); p != "" {
			if v, err := strconv.Atoi(p); err == nil {
				port = v
			}
		}
		out = append(out, Host{
			NodeID: nid, Name: get(row, "name"), IP: get(row, "ip"), Port: port,
			SSHUser: get(row, "ssh_user"), SSHKey: get(row, "ssh_key"), SSHPass: get(row, "ssh_pass"),
		})
	}
	return out, nil
}

// Endpoint — 인벤토리 적재용 **안전 부분집합**(node_id·name·ip·port). 비밀은 담기지 않는다(타입상 불가).
func (h Host) Endpoint() *inventoryv1.MachineEndpoint {
	return &inventoryv1.MachineEndpoint{NodeId: h.NodeID, Name: h.Name, Ip: h.IP, Port: uint32(h.Port)}
}

// Endpoints — 여러 Host의 안전 부분집합(인벤토리 재사용·표시·수정 대상).
func Endpoints(hosts []Host) []*inventoryv1.MachineEndpoint {
	out := make([]*inventoryv1.MachineEndpoint, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, h.Endpoint())
	}
	return out
}

// RenderAnsibleInventory — hosts에서 **런타임 전용** Ansible 인벤토리(ini)를 생성한다.
// 비밀(user/key/pass)이 여기 실린다 — 이 결과는 실행에만 쓰고 **영속하지 않는다**(pqcota 인벤토리엔 Endpoint만).
func RenderAnsibleInventory(hosts []Host) string {
	var b strings.Builder
	b.WriteString("# pqcota 생성(런타임 전용·미영속): discovery 대상\n[targets]\n")
	for _, h := range hosts {
		fmt.Fprintf(&b, "%s ansible_host=%s ansible_port=%d", h.NodeID, h.IP, h.Port)
		if h.SSHUser != "" {
			fmt.Fprintf(&b, " ansible_user=%s", h.SSHUser)
		}
		if h.SSHKey != "" {
			fmt.Fprintf(&b, " ansible_ssh_private_key_file=%s", h.SSHKey)
		}
		if h.SSHPass != "" {
			fmt.Fprintf(&b, " ansible_ssh_pass=%s", h.SSHPass)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
