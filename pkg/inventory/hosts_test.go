package inventory_test

import (
	"strings"
	"testing"

	"github.com/randyinthedev-hash/pqcota/pkg/inventory"
	"google.golang.org/protobuf/encoding/protojson"
)

const hostsCSV = `node_id,name,ip,port,ssh_user,ssh_key
web-01,Web Server 1,10.0.0.2,22,deploy,/home/u/.ssh/secret_key
db-01,DB,10.0.0.3,2222,deploy,/home/u/.ssh/secret_key
# ignored comment
`

func TestParseHostsAndSecretBoundary(t *testing.T) {
	hosts, err := inventory.ParseHosts(strings.NewReader(hostsCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 {
		t.Fatalf("host count: %d (want 2)", len(hosts))
	}
	if hosts[0].NodeID != "web-01" || hosts[0].IP != "10.0.0.2" || hosts[0].Port != 22 || hosts[1].Port != 2222 {
		t.Errorf("wrong parse: %+v", hosts)
	}

	// 핵심: 인벤토리 적재 부분집합(Endpoint)에는 비밀이 없어야 한다 — 직렬화해도 키 경로가 안 나옴.
	for _, h := range hosts {
		b, _ := protojson.Marshal(h.Endpoint())
		if strings.Contains(string(b), "secret_key") || strings.Contains(string(b), "deploy") {
			t.Errorf("a secret leaked into the ingest subset: %s", b)
		}
	}

	// Ansible 인벤토리(런타임 전용)에는 비밀이 실린다.
	inv := inventory.RenderAnsibleInventory(hosts)
	if !strings.Contains(inv, "ansible_ssh_private_key_file=/home/u/.ssh/secret_key") {
		t.Errorf("the Ansible inventory has no key:\n%s", inv)
	}
	if !strings.Contains(inv, "web-01 ansible_host=10.0.0.2 ansible_port=22") {
		t.Errorf("wrong Ansible inventory format:\n%s", inv)
	}
}

// 헤더에 node_id 없으면 에러.
func TestParseHostsNoNodeID(t *testing.T) {
	if _, err := inventory.ParseHosts(strings.NewReader("name,ip\nx,1.2.3.4\n")); err == nil {
		t.Error("a header without node_id must be an error")
	}
}

// os 열 — 빈 칸은 리눅스, 오타는 오류. 조용히 리눅스로 삼키면 Windows 노드에 리눅스
// collector가 올라가고, 실패는 반입이 아니라 실행에서야 드러난다.
func TestParseHostsOSColumn(t *testing.T) {
	hosts, err := inventory.ParseHosts(strings.NewReader(
		"node_id,ip,os\nweb-01,10.0.0.2,\nwin-01,10.0.0.9,Windows\n"))
	if err != nil {
		t.Fatal(err)
	}
	if hosts[0].OS != inventory.OSLinux {
		t.Errorf("an empty os must default to linux: %q", hosts[0].OS)
	}
	if hosts[1].OS != inventory.OSWindows {
		t.Errorf("os is matched case-insensitively: %q", hosts[1].OS)
	}
	if _, err := inventory.ParseHosts(strings.NewReader("node_id,os\nx,windoze\n")); err == nil {
		t.Error("a typo in os must be an error")
	}
}

// OS별 그룹이 나오되 targets는 여전히 전 노드를 가리켜야 한다 —
// `hosts: targets`로 쓰던 플레이북이 그대로 돌아야 하기 때문이다.
func TestRenderAnsibleInventoryGroupsByOS(t *testing.T) {
	hosts, err := inventory.ParseHosts(strings.NewReader(
		"node_id,ip,ssh_user,os\nweb-01,10.0.0.2,deploy,linux\nwin-01,10.0.0.9,deploy,windows\n"))
	if err != nil {
		t.Fatal(err)
	}
	inv := inventory.RenderAnsibleInventory(hosts)
	for _, want := range []string{"[targets:children]", "targets_linux", "targets_windows",
		"[targets_linux]", "[targets_windows]", "win-01 ansible_host=10.0.0.9"} {
		if !strings.Contains(inv, want) {
			t.Errorf("the inventory does not contain %q:\n%s", want, inv)
		}
	}
	// 연결 설정은 그 머신이 정한다 — 지어내지 않고 무엇이 필요한지만 적는다.
	if !strings.Contains(inv, "ansible_connection=winrm") || !strings.Contains(inv, "ansible_shell_type=powershell") {
		t.Errorf("the Windows connection hint is missing:\n%s", inv)
	}
	if strings.Contains(inv, "\nwin-01 ansible_host=10.0.0.9 ansible_port=22 ansible_user=deploy ansible_connection=") {
		t.Error("a connection setting was guessed onto the host line — it must stay a hint")
	}
	// Windows 노드가 없으면 그 그룹도, 안내도 없다.
	only, _ := inventory.ParseHosts(strings.NewReader("node_id,ip\nweb-01,10.0.0.2\n"))
	if inv := inventory.RenderAnsibleInventory(only); strings.Contains(inv, "targets_windows") {
		t.Errorf("an empty group was emitted:\n%s", inv)
	}
}
