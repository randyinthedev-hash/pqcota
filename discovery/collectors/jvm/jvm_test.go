package jvm_test

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/randyinthedev-hash/pqcota/discovery/collectors/jvm"
	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/discovery/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const attachOutput = `PQCOTA_PROVIDERS_BEGIN
0|SUN|21|sun.security.provider.Sun
12|BC|1.79|org.bouncycastle.jce.provider.BouncyCastleProvider
PQCOTA_PROVIDERS_END`

const fallbackOutput = `PQCOTA_STATIC_FALLBACK_BEGIN
evidence_strength=inferred
gap=runtime-introspection
0|SUN
PQCOTA_STATIC_FALLBACK_END`

func TestParseProviders(t *testing.T) {
	c := jvm.ParseProviders(attachOutput)
	if c.Degraded {
		t.Error("attach 성공 출력인데 Degraded=true")
	}
	if len(c.Providers) != 2 || c.Providers[1].Name != "BC" {
		t.Fatalf("providers = %+v, want [SUN BC]", c.Providers)
	}
	if d := jvm.ParseProviders(fallbackOutput); !d.Degraded {
		t.Error("정적 폴백 출력인데 Degraded=false")
	}
}

func TestBuildResult(t *testing.T) {
	res := jvm.BuildResult("cmdb://n1", jvm.ParseProviders(attachOutput))
	cbom := string(res.GetCbomCyclonedx())
	if !strings.Contains(cbom, `"pqcota:crypto_runtime"`) || !strings.Contains(cbom, "SUN,BC") {
		t.Errorf("CycloneDX에 jca provider_set 없음: %s", cbom)
	}
	if len(res.GetCompleteness().GetLayersMissing()) != 0 {
		t.Error("attach 성공인데 갭 존재")
	}
	// 정적 폴백 → JVM_INTROSPECTION 갭
	deg := jvm.BuildResult("cmdb://n1", jvm.ParseProviders(fallbackOutput))
	if len(deg.GetCompleteness().GetLayersMissing()) == 0 {
		t.Error("정적 폴백인데 갭 없음")
	}
}

func TestJvmServiceContract(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	// Runner 주입: 사이드카 실행 대신 픽스처 반환.
	discoveryv1.RegisterCollectorServer(srv, jvm.NewService(
		func(node string, _ map[string]string) (jvm.Collected, error) {
			return jvm.ParseProviders(attachOutput), nil
		}))
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, _ := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()
	cli := discoveryv1.NewCollectorClient(conn)

	caps, err := cli.Describe(context.Background(), &discoveryv1.DescribeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if caps.GetCollectorId() != "jvm-collector" ||
		caps.GetCryptoRuntimes()[0] != commonv1.CryptoRuntime_CRYPTO_RUNTIME_JCA {
		t.Errorf("caps = %+v", caps)
	}

	stream, err := cli.Collect(context.Background(), &discoveryv1.CollectRequest{TargetNodeIds: []string{"cmdb://n1"}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if res.GetEnvelope().GetCollectorId() != "jvm-collector" {
		t.Errorf("collector_id = %q", res.GetEnvelope().GetCollectorId())
	}
	if !strings.Contains(string(res.GetCbomCyclonedx()), "SUN,BC") {
		t.Error("Collect 결과에 provider_set 없음")
	}
}
