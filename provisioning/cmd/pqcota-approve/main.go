// Command pqcota-approve — 확정 계획에 **승인 서명**을 붙인다(§3.3③ finalize 전제).
//
// 계획의 내용 전부(승인 서명 자신만 빼고)를 ed25519로 서명해 `approval_signatures`에 덧붙이고,
// 계획을 다시 낸다. 승인자 id가 서명 문자열 안에 들어가므로, 검증하는 쪽은 **그 사람의 키로만**
// 확인한다(pqcota-provision의 PQCOTA_APPROVAL_KEYS).
//
// usage: pqcota-approve --approver <id> <plan.json>
//
//	--approver <id>          : 승인자 id. ':'는 쓸 수 없다(서명 문자열의 구분자)
//	env PQCOTA_APPROVAL_KEY  : base64 ed25519 **개인키**. pqcota-keygen이 낸 것
//
// 서명한 계획은 stdout으로 나간다. 원본을 덮어쓰지 않는 이유는, 승인 전 계획과 승인된 계획이
// 같은 파일이면 무엇에 서명했는지 되짚을 수 없기 때문이다.
//
// ★ 서명한 뒤에 계획을 고치면 **승인은 무효가 된다**. 그것이 요점이다: 승인자는 계획의 이름이
// 아니라 조치의 내용에 책임을 진다. 조각을 나중에 채우려면(FillPlan) 채운 뒤에 승인받는다.
package main

import (
	"flag"
	"fmt"
	"os"

	provisioningv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/provisioning/v1"
	"github.com/randyinthedev-hash/pqcota/pkg/kernel/sign"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	approver := flag.String("approver", "", "approver id — it goes inside the signature so verification is bound to this person")
	flag.Parse()
	if flag.NArg() < 1 || *approver == "" {
		fmt.Fprintln(os.Stderr, "usage: pqcota-approve --approver <id> <plan.json>   (env PQCOTA_APPROVAL_KEY = base64 ed25519 private key)")
		os.Exit(2)
	}

	key := os.Getenv("PQCOTA_APPROVAL_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "PQCOTA_APPROVAL_KEY is not set — there is no key to approve with. Generate one with pqcota-keygen.")
		os.Exit(2)
	}

	raw, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "reading the plan:", err)
		os.Exit(1)
	}
	plan := &provisioningv1.FinalizedPlan{}
	if err := protojson.Unmarshal(raw, plan); err != nil {
		fmt.Fprintln(os.Stderr, "parsing the plan:", err)
		os.Exit(1)
	}

	sig, err := sign.SignApproval(key, *approver, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "signing:", err)
		os.Exit(1)
	}
	plan.ApprovalSignatures = append(plan.ApprovalSignatures, sig)

	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "writing the plan:", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
	fmt.Fprintf(os.Stderr, "[approve] %s signed plan %q (%d actions). The approval covers every field except the signatures themselves — editing the plan after this invalidates it.\n",
		*approver, plan.GetId(), len(plan.GetActions()))
}
