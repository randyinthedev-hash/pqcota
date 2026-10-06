// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

import (
	"testing"

	"github.com/randyinthedev-hash/pqcota-common/pkg/org"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/history"
	"github.com/randyinthedev-hash/pqcota-provisioning/pkg/provisioning"
)

// TestScopedIsSatisfiedByTheStores — 저장소 인터페이스를 안 건드리고 조직을 물을 수 있다.
func TestScopedIsSatisfiedByTheStores(t *testing.T) {
	var _ org.Scoped = (*history.MemStore)(nil)
	var _ org.Scoped = (*history.PgStore)(nil)
	var _ org.Scoped = (*inventory.MemMetaStore)(nil)
	var _ org.Scoped = (*inventory.PgMetaStore)(nil)
	var _ org.Scoped = (*provisioning.PgRecordStore)(nil)

	// history.Store로 받아 온 것도 타입 단언으로 물을 수 있다 — 인터페이스에 메서드를 더하지
	// 않았으므로 밖의 구현체는 깨지지 않는다.
	var st history.Store = history.NewMemStore()
	sc, ok := st.(org.Scoped)
	if !ok || sc.Org() != org.Default {
		t.Fatalf("a Store cannot be asked through org.Scoped: ok=%v", ok)
	}
}
