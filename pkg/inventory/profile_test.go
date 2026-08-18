package inventory_test

import (
	"strings"
	"testing"

	inventoryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/inventory/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/inventory"
)

func TestParseProfiles(t *testing.T) {
	csv := `node_id,display_name,environment,role,owner,location,labels
node-db,결제 DB,production,db,DBA팀,seoul-dc,compliance=pci;tier=1
# 주석 행은 무시
node-web,결제 웹,dev,web,플랫폼팀,,
`
	profs, err := inventory.ParseProfiles(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(profs) != 2 {
		t.Fatalf("프로필 %d개(2 기대)", len(profs))
	}
	db := profs[0]
	if db.GetNodeId() != "node-db" || db.GetDisplayName() != "결제 DB" || db.GetRole() != "db" {
		t.Errorf("db 필드: %+v", db)
	}
	if db.GetEnvironment() != inventoryv1.Environment_ENVIRONMENT_PRODUCTION {
		t.Errorf("environment: %v", db.GetEnvironment())
	}
	if db.GetLabels()["compliance"] != "pci" || db.GetLabels()["tier"] != "1" {
		t.Errorf("labels: %v", db.GetLabels())
	}
	if db.GetSource() != inventoryv1.ProfileSource_PROFILE_SOURCE_CMDB {
		t.Errorf("source: %v", db.GetSource())
	}
	if profs[1].GetEnvironment() != inventoryv1.Environment_ENVIRONMENT_DEVELOPMENT {
		t.Errorf("dev 별칭 파싱 실패: %v", profs[1].GetEnvironment())
	}
}
