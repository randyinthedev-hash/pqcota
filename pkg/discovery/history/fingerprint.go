package history

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
)

// ContentHash — 스냅샷의 **실질 내용** 지문. "같은 상태를 다시 관측한 것인가"를 판정하는 근거다.
// 포함/제외 필드의 근거 표는 인벤토리 설계 §7.3에 있다.
//
// 휘발 필드는 반드시 제외한다 — 관측할 때마다 달라지므로 포함하면 항상 "변화"가 되어
// 중복 억제가 무력해진다:
//   - Finding.derived_from_snapshot_id·ruleset_version (스냅샷마다 다름)
//   - ObservedEdge.observed_count·first_seen·last_seen (관측 빈도·시각)
//
// 반대로 실질 내용(자산 동일성·버전·바인딩·귀속·협상 그룹·완전성 갭)은 전부 포함한다.
// 여기서 빠뜨린 필드가 바뀌면 "변화 없음"으로 접혀 이력에서 사라지므로, 필드를 추가할 땐
// 이 함수도 함께 갱신해야 한다.
func ContentHash(s *Snapshot) string {
	h := sha256.New()

	fs := append([]*discoveryv1.Finding(nil), s.Findings...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].GetId() < fs[j].GetId() })
	for _, f := range fs {
		fmt.Fprintf(h, "F|%s|%d|%d|%d|%d|%s|%s|%s|%s|%s\n",
			f.GetId(), f.GetCryptoRuntime(), f.GetUsageContext(), f.GetDetectionMethod(),
			f.GetEvidenceStrength(), f.GetAlgorithm(), f.GetPqcReadiness(),
			f.GetFipsValidation(), f.GetRemediationClass(), sortedJoin(f.GetAppKeys()))
		if o := f.GetOpenssl(); o != nil {
			fmt.Fprintf(h, "  O|%s|%s|%s|%d\n", o.GetLib(), o.GetFork(), o.GetVersion(), o.GetBindingMode())
		}
		if j := f.GetJca(); j != nil {
			// provider_set은 **순서가 의미를 갖는다**(우선순위 협상, §1.2) — 정렬하지 않는다.
			fmt.Fprintf(h, "  J|%s|%s|%s|%d\n", j.GetJdkVendor(), j.GetJdkVersion(),
				strings.Join(j.GetProviderSet(), ","), j.GetRegistrationMode())
		}
	}

	es := append([]*discoveryv1.ObservedEdge(nil), s.Edges...)
	sort.Slice(es, func(i, j int) bool { return edgeKey(es[i]) < edgeKey(es[j]) })
	for _, e := range es {
		fmt.Fprintf(h, "E|%s|%s|%s\n", edgeKey(e), e.GetNegotiatedGroup(), e.GetCipher())
	}

	if c := s.Completeness; c != nil {
		var miss []string
		for _, l := range c.GetLayersMissing() {
			miss = append(miss, l.String())
		}
		sort.Strings(miss)
		fmt.Fprintf(h, "C|%s|%s\n", strings.Join(miss, ","), c.GetNote())
	}
	return hex.EncodeToString(h.Sum(nil))
}

// edgeKey — 엣지의 동일성 키(빈도·시각 제외).
func edgeKey(e *discoveryv1.ObservedEdge) string {
	return fmt.Sprintf("%s>%s@%s:%d/%d/%d/%d", e.GetSrcNodeId(), e.GetDstNodeId(), e.GetDstAddr(),
		e.GetPort(), e.GetProtocol(), e.GetRole(), e.GetDetectionMethod())
}

func sortedJoin(ss []string) string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return strings.Join(out, ",")
}
