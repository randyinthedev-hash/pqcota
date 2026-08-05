// Package deploy generates collector deployment artifacts (설계 §2.4 배포 티어).
// 자체 원격 실행 엔진을 만들지 않는다(§4.6) — 사용자의 기존 substrate(Ansible)가 실행하도록
// 플레이북을 "생성"만 한다. 대상은 스코프 마스터 게이트 통과 노드에 한정(§0.4).
package provisioning

import (
	"fmt"
	"strings"
)

// GeneratePlaybook — T2(substrate-반자동): collector 바이너리를 대상 노드에 복사·실행하고
// 결과를 회수하는 Ansible 플레이북 YAML을 생성한다. 실행 주체는 사용자(§2.4 T2).
//
// nodes는 이미 스코프 마스터로 게이트된 것이어야 한다(§0.4는 코어 책임).
func GeneratePlaybook(nodes []string, collectorBin, remotePath string) string {
	var b strings.Builder
	b.WriteString("# 생성됨: pqcota T2 배포 플레이북 (사용자 Ansible이 실행)\n")
	b.WriteString("# 자체 push 엔진 아님 — 검증된 substrate 재사용(§4.6). 대상=스코프 게이트 노드(§0.4).\n")
	b.WriteString("- name: pqcota openssl-collector 배포·수집\n")
	b.WriteString("  hosts:\n")
	for _, n := range nodes {
		fmt.Fprintf(&b, "    - %s\n", n)
	}
	b.WriteString("  become: true   # /proc 타 프로세스 접근 위해 (CAP_SYS_PTRACE/root)\n")
	b.WriteString("  tasks:\n")
	fmt.Fprintf(&b, "    - name: collector 바이너리 배치\n")
	fmt.Fprintf(&b, "      ansible.builtin.copy:\n")
	fmt.Fprintf(&b, "        src: %s\n", collectorBin)
	fmt.Fprintf(&b, "        dest: %s\n", remotePath)
	fmt.Fprintf(&b, "        mode: '0755'\n")
	fmt.Fprintf(&b, "    - name: collector 실행(읽기전용 수집)\n")
	fmt.Fprintf(&b, "      ansible.builtin.command: %s\n", remotePath)
	fmt.Fprintf(&b, "      register: pqcota_out\n")
	fmt.Fprintf(&b, "    - name: 결과 회수(서명 리포트)\n")
	fmt.Fprintf(&b, "      ansible.builtin.debug:\n")
	fmt.Fprintf(&b, "        var: pqcota_out.stdout\n")
	return b.String()
}
