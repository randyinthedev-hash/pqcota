// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package crossstage_test

// jvm 수집기의 배포본 결과(discovery)를 정규화(inventory)에 잇는다(v0.12.0 JCA 식별 1단계).
// 수집기 영역은 정규화를 import하지 않으므로(checkdeps R5) 두 단계를 함께 재는 시험은 여기에 둔다.

import (
	"errors"
	"strings"
	"testing"

	commonv1 "github.com/randyinthedev-hash/pqcota-common/gen/pqcota/common/v1"
	"github.com/randyinthedev-hash/pqcota-discovery/collectors/jvm"
	"github.com/randyinthedev-hash/pqcota-inventory/pkg/inventory/normalize"
)

func jvmChain(degraded bool, names ...string) *jvm.Collected {
	c := &jvm.Collected{Degraded: degraded, Raw: "raw:" + strings.Join(names, ",")}
	for i, n := range names {
		p := jvm.Provider{Order: i + 1, Name: n}
		if !degraded {
			p.ServicesSeen = true
			if n == "BC" {
				p.PQCServices = []string{"ML-DSA-65", "ML-KEM-768", "SLH-DSA-SHA2-128S"}
			}
		}
		c.Providers = append(c.Providers, p)
	}
	return c
}

// 런타임 관측과 정적 폴백이 같은 체인을 보고하고, 같은 배포본의 다른 JVM 하나는 체인을 얻지 못했다.
//   - 관측한 둘은 같은 변형에 묶이고 두 모드의 수가 각각 남는다.
//   - 못 본 JVM은 따로 변형으로 남는다.
//   - finding의 provider_set은 비어 있다.
func TestCollectorToNormalizeMixedObservation(t *testing.T) {
	j := jvm.JVMProc{JavaHome: "/opt/jdk", Cmdline: "/opt/jdk/bin/java\x00-jar\x00/opt/pay/app.jar\x00"}
	members := []jvm.Member{
		{JVM: j, Collected: jvmChain(false, "SUN", "SunRsaSign", "BC")},
		{JVM: j, Collected: jvmChain(true, "SUN", "SunRsaSign", "BC")},
		{JVM: j, Err: errors.New("attach refused")},
	}
	r := jvm.BuildDeploymentResult("pay", jvm.GroupDeployments(members)[0])
	fs, err := normalize.DeriveFindings(r, "s", normalize.RulesetVersion)
	if err != nil || len(fs) != 1 {
		t.Fatalf("derive: %v %d", err, len(fs))
	}
	jca := fs[0].GetJca()
	if len(jca.GetProviderSet()) != 0 || len(jca.GetInstances()) != 2 || jca.GetAppKeyKind() != jvm.KindJarPath {
		t.Fatalf("provider_set %v, %d instances, kind %q", jca.GetProviderSet(), len(jca.GetInstances()), jca.GetAppKeyKind())
	}
	for _, in := range jca.GetInstances() {
		modes := map[commonv1.JcaRegistrationMode]uint32{}
		for _, mc := range in.GetModeCounts() {
			modes[mc.GetMode()] = mc.GetCount()
		}
		if in.GetChainObserved() {
			if in.GetCount() != 2 || modes[commonv1.JcaRegistrationMode_JCA_REGISTRATION_MODE_STATIC] != 1 ||
				modes[commonv1.JcaRegistrationMode_JCA_REGISTRATION_MODE_DYNAMIC] != 1 ||
				in.GetRegistrationMode() != commonv1.JcaRegistrationMode_JCA_REGISTRATION_MODE_UNSPECIFIED {
				t.Errorf("observed variant: count %d modes %v mode %v", in.GetCount(), modes, in.GetRegistrationMode())
			}
		} else if in.GetCount() != 1 || len(in.GetProviderSet()) != 0 || in.GetPqcReadiness() != "unknown" {
			t.Errorf("unobserved variant: count %d chain %v readiness %q", in.GetCount(), in.GetProviderSet(), in.GetPqcReadiness())
		}
	}
	if !strings.HasPrefix(fs[0].GetPqcReadiness(), "varies (") {
		t.Errorf("readiness = %q", fs[0].GetPqcReadiness())
	}
	// 정규화는 컴포넌트 이름의 키를 그대로 id에 쓴다. 수집기가 키를 줄이지 않았는지 이름에서 본다.
	if !strings.Contains(string(r.GetCbomCyclonedx()), `"jca-provider-chain@/opt/pay/app.jar"`) {
		t.Errorf("component name: %s", r.GetCbomCyclonedx())
	}
}
