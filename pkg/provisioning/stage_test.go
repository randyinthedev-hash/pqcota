package provisioning_test

import (
	"strings"
	"testing"

	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	provisioningv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/provisioning/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/provisioning"
	"gopkg.in/yaml.v3"
)

// 오류 메시지용 표시 이름 — 검증 대상이 아니라 어느 레벨에서 깨졌는지 읽기 위한 것이다.
func levelName(l provisioningv1.DeployAutomationLevel) string {
	switch l {
	case provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L1_STAGE_ONLY:
		return "L1"
	case provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL:
		return "L2"
	case provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L3_FULL_AUTO:
		return "L3"
	}
	return l.String()
}

func samplePlan() *provisioningv1.FinalizedPlan {
	return &provisioningv1.FinalizedPlan{Actions: []*provisioningv1.RemediationAction{
		{Id: "a1", TargetNodeId: "web-01", CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
			Kind: provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY, TargetAlgorithm: "ML-KEM (FIPS 203)"},
		{Id: "a2", TargetNodeId: "app-01", CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA,
			Kind: provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, TargetAlgorithm: "ML-KEM (FIPS 203)", ProviderChoice: "BC"},
		{Id: "a3", TargetNodeId: "db-01", CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
			Kind: provisioningv1.RemediationKind_REMEDIATION_KIND_FORK_REPLACE},
	}}
}

// L2: 노드별 play + 모듈 스테이지(주입형) + config 조각 배치. 활성화·재시작은 하지 않는다 — 그건 L3.
func TestProvisioningPlaybookL2(t *testing.T) {
	pb := provisioning.GenerateProvisioningPlaybook(samplePlan(), provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL)
	for _, want := range []string{
		`hosts: ["web-01"]`, `hosts: ["app-01"]`,
		"provider 모듈 스테이지 (BC)", "/opt/pqcota/BC.jar", // JCA 주입 모듈
		"content: |",           // config 조각 배치
		"java.security.pqcota", // JCA config 경로
		"openssl-pqc.cnf",      // openssl config 경로
		"a3(REMEDIATION_KIND_FORK_REPLACE): config로 배포 불가", // 비-config 조치는 주석
	} {
		if !strings.Contains(pb, want) {
			t.Errorf("L2 플레이북에 %q 없음:\n%s", want, pb)
		}
	}
	if strings.Contains(pb, "restart") || strings.Contains(pb, "재시작:") {
		t.Errorf("L2에 재시작이 있으면 안 됨(재시작은 L3의 restart 훅):\n%s", pb)
	}
}

// JCA provider 주입이 있으면 헤더에 classpath 배선 함정을 먼저 짚는다(JAR 배치≠로드).
// openssl 전용 계획엔 뜨지 않는다 — 무관한 노트로 헤더를 어지럽히지 않는다.
func TestJCAClasspathHintInHeader(t *testing.T) {
	// samplePlan은 JCA 주입(a2)을 포함 → 헤더 노트가 떠야 한다(L1·L2 모두).
	for _, lvl := range []provisioningv1.DeployAutomationLevel{
		provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L1_STAGE_ONLY,
		provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL,
	} {
		pb := provisioning.GenerateProvisioningPlaybook(samplePlan(), lvl)
		for _, want := range []string{"JCA provider 주입 포함", "classpath 또는 --module-path", "activation.activate"} {
			if !strings.Contains(pb, want) {
				t.Errorf("lvl=%s: JCA 함정 헤더에 %q 없음:\n%s", levelName(lvl), want, pb)
			}
		}
	}

	// openssl 전용 계획 → 노트 없음.
	ossl := &provisioningv1.FinalizedPlan{Actions: []*provisioningv1.RemediationAction{
		{Id: "a1", TargetNodeId: "web-01", CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
			Kind: provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, ProviderChoice: "oqsprovider"},
	}}
	if pb := provisioning.GenerateProvisioningPlaybook(ossl, provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL); strings.Contains(pb, "JCA provider 주입 포함") {
		t.Errorf("openssl 전용인데 JCA 노트가 떴다:\n%s", pb)
	}
}

// L1: 모듈 스테이지만 — config 조각 배치 없음.
func TestProvisioningPlaybookL1(t *testing.T) {
	pb := provisioning.GenerateProvisioningPlaybook(samplePlan(), provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L1_STAGE_ONLY)
	if !strings.Contains(pb, "/opt/pqcota/BC.jar") {
		t.Error("L1도 모듈 스테이지는 해야")
	}
	if strings.Contains(pb, "content: |") {
		t.Errorf("L1(stage-only)은 config 조각을 배치하지 않아야:\n%s", pb)
	}
}

// L3 — 사용자가 준 훅을 **의미 순서로** 배치한다: pre → [배치] → activate → restart.
// 순서가 곧 안전성이다(내리고 → 바꾸고 → 참조되게 하고 → 새로 로드).
func TestL3ActivationOrder(t *testing.T) {
	a := &provisioningv1.RemediationAction{
		Id: "a1", TargetNodeId: "db-01", CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		Kind: provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, ProviderChoice: "oqsprovider",
		Activation: &provisioningv1.ActivationHooks{
			Pre: "systemctl stop payment", Activate: "ln -sf /etc/pqcota/openssl-pqc.cnf /etc/ssl/inc/",
			Deactivate: "rm -f /etc/ssl/inc/openssl-pqc.cnf", Restart: "systemctl start payment",
		},
	}
	plan := &provisioningv1.FinalizedPlan{
		Status:             provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED,
		ApprovalSignatures: []string{"r"}, Actions: []*provisioningv1.RemediationAction{a},
	}
	L3 := provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L3_FULL_AUTO

	fwd := provisioning.GenerateProvisioningPlaybook(plan, L3)
	iPre, iCfg := strings.Index(fwd, "systemctl stop payment"), strings.Index(fwd, "config 조각 배치")
	iAct, iRst := strings.Index(fwd, "ln -sf"), strings.Index(fwd, "systemctl start payment")
	for _, s := range []int{iPre, iCfg, iAct, iRst} {
		if s < 0 {
			t.Fatalf("L3 플레이북에 단계가 빠졌다:\n%s", fwd)
		}
	}
	if !(iPre < iCfg && iCfg < iAct && iAct < iRst) {
		t.Errorf("순서가 pre→배치→activate→restart여야: pre=%d cfg=%d act=%d restart=%d", iPre, iCfg, iAct, iRst)
	}

	// 롤백은 정확한 역순 — 내리고 → 활성화 되돌리고 → 파일 제거 → 재시작.
	back := provisioning.GenerateRollbackPlaybook(plan, L3)
	jPre, jDeact := strings.Index(back, "systemctl stop payment"), strings.Index(back, "rm -f /etc/ssl/inc")
	jRm, jRst := strings.Index(back, "state: absent"), strings.Index(back, "systemctl start payment")
	if !(jPre < jDeact && jDeact < jRm && jRm < jRst) {
		t.Errorf("롤백 순서가 pre→deactivate→제거→restart여야: %d %d %d %d", jPre, jDeact, jRm, jRst)
	}

	// L2에는 훅이 들어가지 않는다 — 활성화는 L3에서만.
	l2 := provisioning.GenerateProvisioningPlaybook(plan, provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL)
	if strings.Contains(l2, "systemctl") || strings.Contains(l2, "ln -sf") {
		t.Errorf("L2에 활성화 훅이 새어들었다:\n%s", l2)
	}
}

// ★ 추측 금지(§2.5) — 훅이 비면 그 단계를 **만들지 않고**, 무엇이 안 일어나는지 고지한다.
func TestL3MissingHooksWarnButDoNotGuess(t *testing.T) {
	bare := &provisioningv1.RemediationAction{
		Id: "a1", TargetNodeId: "db-01", CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		Kind: provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY,
	}
	plan := &provisioningv1.FinalizedPlan{
		Status:             provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED,
		ApprovalSignatures: []string{"r"}, Actions: []*provisioningv1.RemediationAction{bare},
	}
	L3 := provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L3_FULL_AUTO

	// 활성화 명령을 지어내지 않는다 — systemctl 같은 걸 추측하면 남의 운영을 망가뜨린다.
	out := provisioning.GenerateProvisioningPlaybook(plan, L3)
	if strings.Contains(out, "systemctl") || strings.Contains(out, "ansible.builtin.shell") {
		t.Errorf("빈 훅에서 명령을 지어냈다:\n%s", out)
	}
	// 대신 무엇이 일어나지 않는지 경고한다.
	w := provisioning.ActivationWarnings(plan, L3)
	if len(w) < 2 {
		t.Fatalf("activate·restart 부재를 각각 고지해야: %v", w)
	}
	joined := strings.Join(w, " ")
	for _, want := range []string{"activate", "restart"} {
		if !strings.Contains(joined, want) {
			t.Errorf("경고에 %q 없음: %v", want, w)
		}
	}
	// L1/L2에서는 훅 경고가 없다(활성화를 약속하지 않으므로).
	if len(provisioning.ActivationWarnings(plan, provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL)) != 0 {
		t.Error("L2에서 활성화 경고가 나오면 안 된다")
	}
	// deactivate 경고는 activate가 있을 때만 의미 있다(되돌릴 게 있을 때).
	bare.Activation = &provisioningv1.ActivationHooks{Activate: "x", Restart: "y"}
	if got := strings.Join(provisioning.ActivationWarnings(plan, L3), " "); !strings.Contains(got, "deactivate") {
		t.Errorf("activate만 있고 deactivate 없으면 가역성 경고가 있어야: %q", got)
	}
}

// 같은 노드에 조치가 여러 개고 재시작 명령이 같으면 — 재시작은 **한 번**이어야 한다.
// 조치별로 훅을 내면 서비스를 n번 흔들고, 활성화 사이에 재시작이 끼어 일부만 반영된 채 뜬다.
func TestL3HooksGroupedAndDeduped(t *testing.T) {
	h := func() *provisioningv1.ActivationHooks {
		return &provisioningv1.ActivationHooks{
			Pre: "svc stop pay", Activate: "", Deactivate: "", Restart: "svc start pay",
		}
	}
	a1 := &provisioningv1.RemediationAction{Id: "a1", TargetNodeId: "db-01",
		CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		Kind:          provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY, Activation: h()}
	a2 := &provisioningv1.RemediationAction{Id: "a2", TargetNodeId: "db-01",
		CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		Kind:          provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY, Activation: h()}
	a2.Activation.Activate = "touch /etc/ssl/inc/on" // a2만 활성화 명령이 다름
	plan := &provisioningv1.FinalizedPlan{
		Status:             provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED,
		ApprovalSignatures: []string{"r"}, Actions: []*provisioningv1.RemediationAction{a1, a2},
	}
	L3 := provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L3_FULL_AUTO

	for name, pb := range map[string]string{
		"forward":  provisioning.GenerateProvisioningPlaybook(plan, L3),
		"rollback": provisioning.GenerateRollbackPlaybook(plan, L3),
	} {
		if n := strings.Count(pb, "svc start pay"); n != 1 {
			t.Errorf("%s: 같은 재시작 명령이 %d번(1이어야):\n%s", name, n, pb)
		}
		if n := strings.Count(pb, "svc stop pay"); n != 1 {
			t.Errorf("%s: 같은 pre 명령이 %d번(1이어야)", name, n)
		}
		// 재시작은 활성화보다 **뒤**에 한 번 — 사이에 끼지 않는다.
		if i, j := strings.Index(pb, "touch /etc/ssl/inc/on"), strings.Index(pb, "svc start pay"); name == "forward" && !(i > 0 && i < j) {
			t.Errorf("forward: 활성화(%d)가 재시작(%d)보다 앞이어야", i, j)
		}
	}
}

// 같은 노드·같은 런타임에 **서로 다른** 조각이 둘이면, 같은 경로에 두 번 copy해선 안 된다 —
// 뒤가 앞을 조용히 덮어써 앞 조치가 사라진다(모듈만 놓이고 참조는 안 되는 상태로 배포됨, §2.6).
func TestConfigFragmentsNeverOverwriteEachOther(t *testing.T) {
	mk := func(id, kind string) *provisioningv1.RemediationAction {
		k := provisioningv1.RemediationKind_REMEDIATION_KIND_CONFIG_ONLY
		prov := ""
		if kind == "inject" {
			k, prov = provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT, "oqsprovider"
		}
		return &provisioningv1.RemediationAction{Id: id, TargetNodeId: "db-01",
			CryptoRuntime: commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL, Kind: k,
			ProviderChoice: prov, TargetAlgorithm: "ML-KEM (FIPS 203)"}
	}
	// a1=주입(provider_sect 있는 조각), a2=config-only(다른 조각) → 내용이 다르다.
	plan := &provisioningv1.FinalizedPlan{Actions: []*provisioningv1.RemediationAction{mk("a1", "inject"), mk("a2", "cfg")}}
	L2 := provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL

	pb := provisioning.GenerateProvisioningPlaybook(plan, L2)
	if n := strings.Count(pb, `dest: "`+provisioning.OpenSSLConfigPath+`"`); n > 1 {
		t.Errorf("같은 경로에 %d번 배치 — 뒤가 앞을 덮어쓴다:\n%s", n, pb)
	}
	// 두 조각이 모두 살아 있어야 한다(하나가 사라지면 안 된다).
	for _, want := range []string{"provider_sect", "config-only"} {
		if !strings.Contains(pb, want) {
			t.Errorf("조각 %q가 유실됐다:\n%s", want, pb)
		}
	}
	// 경로를 나눈 사실을 알린다 — 나눈 채 두면 어느 것도 참조되지 않는다.
	if w := provisioning.ConfigConflictWarnings(plan); len(w) != 1 || !strings.Contains(w[0], "activation.activate") {
		t.Errorf("경로 분리를 고지해야: %v", w)
	}
	// 롤백은 나눈 경로를 **그대로** 지운다(대칭) — 안 지우면 잔재가 남는다.
	back := provisioning.GenerateRollbackPlaybook(plan, L2)
	for id, d := range provisioning.ConfigDests(plan.GetActions()) {
		if !strings.Contains(back, `path: "`+d+`",`) {
			t.Errorf("롤백이 %s(조치 %s)를 지우지 않는다:\n%s", d, id, back)
		}
	}

	// 내용이 같은 조각 둘 → 경로를 나누지 않고, 배치도 한 번(중복 파일·중복 태스크 없음).
	same := &provisioningv1.FinalizedPlan{Actions: []*provisioningv1.RemediationAction{mk("b1", "cfg"), mk("b2", "cfg")}}
	sp := provisioning.GenerateProvisioningPlaybook(same, L2)
	if n := strings.Count(sp, "config 조각 배치"); n != 1 {
		t.Errorf("동일 조각인데 배치 태스크가 %d개(1이어야):\n%s", n, sp)
	}
	if len(provisioning.ConfigConflictWarnings(same)) != 0 {
		t.Error("동일 조각엔 충돌 경고가 없어야")
	}
}

// 생성물이 **정말 YAML인가** — 문자열 검사로는 못 잡는다. 사용자가 적은 훅에는 줄바꿈·`:`·`#`·
// 인용부호가 들어올 수 있고, 그대로 한 줄 스칼라에 붙이면 ansible-playbook이 파일을 읽지도 못한다
// 여러 줄 명령·따옴표·주석 기호가 훅에 들어오면 특히 위험하다. 그래서 실제로 파싱한다.
func TestGeneratedPlaybooksAreValidYAML(t *testing.T) {
	nasty := "printf 'OPENSSL_CONF=%s\\n' /etc/pqcota/x.cnf > /etc/pqcota/service.env\nsystemctl daemon-reload  # 주석: 콜론도 있다"
	a := &provisioningv1.RemediationAction{
		Id: "a1: 이상한 id", TargetNodeId: "db-01",
		CryptoRuntime:  commonv1.CryptoRuntime_CRYPTO_RUNTIME_OPENSSL,
		Kind:           provisioningv1.RemediationKind_REMEDIATION_KIND_PROVIDER_INJECT,
		ProviderChoice: `oqs "prov": #1`, TargetAlgorithm: "ML-KEM (FIPS 203)",
		Activation: &provisioningv1.ActivationHooks{
			Pre: "svc stop", Activate: nasty, Deactivate: "rm -f /etc/pqcota/service.env", Restart: "svc start",
		},
	}
	plan := &provisioningv1.FinalizedPlan{
		Status:             provisioningv1.PlanStatus_PLAN_STATUS_FINALIZED,
		ApprovalSignatures: []string{"r"}, Actions: []*provisioningv1.RemediationAction{a},
	}

	for _, lvl := range []provisioningv1.DeployAutomationLevel{
		provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L1_STAGE_ONLY,
		provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L2_STAGE_INSTALL,
		provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L3_FULL_AUTO,
	} {
		for name, pb := range map[string]string{
			"forward":  provisioning.GenerateProvisioningPlaybook(plan, lvl),
			"rollback": provisioning.GenerateRollbackPlaybook(plan, lvl),
		} {
			var plays []map[string]any
			if err := yaml.Unmarshal([]byte(pb), &plays); err != nil {
				t.Fatalf("%s/%s: 생성물이 YAML이 아니다: %v\n%s", levelName(lvl), name, err, pb)
			}
			if len(plays) != 1 {
				t.Fatalf("%s/%s: play 1개여야, %d개", levelName(lvl), name, len(plays))
			}
			if got := plays[0]["hosts"]; !strings2Contains(got, "db-01") {
				t.Errorf("%s/%s: hosts가 온전하지 않다: %#v", levelName(lvl), name, got)
			}
		}
	}

	// L3 forward에는 훅 명령이 **원문 그대로** 살아 있어야 한다(이스케이프로 변형되면 다른 명령이 된다).
	fwd := provisioning.GenerateProvisioningPlaybook(plan, provisioningv1.DeployAutomationLevel_DEPLOY_AUTOMATION_LEVEL_L3_FULL_AUTO)
	var plays []struct {
		Tasks []map[string]any `yaml:"tasks"`
	}
	if err := yaml.Unmarshal([]byte(fwd), &plays); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, tk := range plays[0].Tasks {
		if s, ok := tk["ansible.builtin.shell"].(string); ok && strings.TrimSpace(s) == strings.TrimSpace(nasty) {
			found = true
		}
	}
	if !found {
		t.Errorf("훅 명령이 원문 그대로 실리지 않았다:\n%s", fwd)
	}
}

// hosts는 [문자열] 형태 — 파싱 결과에서 노드 id를 찾는다.
func strings2Contains(v any, want string) bool {
	if l, ok := v.([]any); ok {
		for _, e := range l {
			if s, ok := e.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}
