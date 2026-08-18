package provisioning_test

import (
	"strings"
	"testing"

	"github.com/randyinthedev-hash/pqcota/pkg/provisioning"
)

func TestGeneratePlaybook(t *testing.T) {
	yaml := provisioning.GeneratePlaybook([]string{"host-a", "host-b"}, "./openssl-collector", "/opt/pqcota/collector")
	for _, want := range []string{"- host-a", "- host-b", "ansible.builtin.copy", "/opt/pqcota/collector", "become: true"} {
		if !strings.Contains(yaml, want) {
			t.Errorf("플레이북에 %q 없음:\n%s", want, yaml)
		}
	}
}
