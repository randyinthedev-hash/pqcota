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
