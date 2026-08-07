// Command pqcota-jvmscan — 타깃(Java) 노드에서 실행. 실 JVM에 java.security 프로바이더 체인을
// 조회(Security.getProviders())해 JCA 자산을 CollectionResult JSON으로 낸다.
//
// 먼저 이 머신의 **실행 중 JVM을 정찰**한다(/proc 스캔, openssl과 대칭) — 발견된 JVM·JDK를
// stderr로 보이고, JAVA_BIN이 없으면 그 java 바이너리를 기본값으로 쓴다. 즉 호출자가 JDK
// 경로를 미리 몰라도 된다. (현 관측은 프로브 JVM으로 정적 등록 체인을 보는 경량 경로 — 실행 중
// 앱의 동적 addProvider까지 보려면 사이드카 attach 경로가 필요하다, §2.2.)
//
// usage: pqcota-jvmscan [--output json|table] [--pid N] [node-id]
//
//	--output json (기본) CollectionResult JSON을 stdout에 — 중앙이 회수해 적재한다
//	        table       정규화까지 하고 사람이 읽는 표를 stdout에 — 저장하지 않는다
//	--pid N             그 PID의 JVM 하나만 관측(기본: 정찰로 찾은 전부)
//	--recon             정찰만 → 발견 JVM JSON(배포 결정용, 관측 안 함)
//
// env: JAVA_BIN(미지정 시 정찰로 발견한 java, 없으면 PATH의 java), JVMSCAN_CP(bcprov 등 classpath)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/pqcota/pqcota/discovery/cmd/internal/localview"
	"github.com/pqcota/pqcota/discovery/collectors/jvm"
	discoveryv1 "github.com/pqcota/pqcota/gen/pqcota/discovery/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// Java 11+ 단일 파일 소스 실행으로 프로바이더 체인을 "N|name|version|class"로 덤프한다
// (collectors/jvm.ParseProviders 입력 형식과 일치).
const probeSrc = `import java.security.*;
public class Probe {
  public static void main(String[] a){
    Provider[] ps = Security.getProviders();
    for (int i=0;i<ps.length;i++){
      Provider p = ps[i];
      String v;
      try { v = p.getVersionStr(); } catch (Throwable t){ v = String.valueOf(p.getVersion()); }
      System.out.println((i+1)+"|"+p.getName()+"|"+v+"|"+p.getClass().getName());
    }
  }
}
`

func main() {
	recon := flag.Bool("recon", false, "정찰만 → 발견 JVM JSON(관측 안 함)")
	pidOnly := flag.Int("pid", 0, "이 PID의 JVM 하나만 관측(0이면 정찰로 찾은 전부)")
	out := flag.String("output", "json", "출력 형식: json | table")
	flag.Parse()
	if *out != "json" && *out != "table" {
		fmt.Fprintf(os.Stderr, "알 수 없는 --output %q — json | table\n", *out)
		os.Exit(2)
	}

	// --recon: 정찰만 하고 발견된 JVM을 JSON으로 낸다(관측 안 함). **배포 결정용** —
	// 오케스트레이터(Ansible 등)가 "이 노드에 JVM이 있나"를 보고 에이전트 JAR를 보낼지 정한다.
	// JVM이 없으면 `[]` — 없는 노드에 Java 애드온을 뿌리지 않기 위한 근거다(배포 설계 §2).
	if *recon {
		emitRecon()
		return
	}

	node := flag.Arg(0)
	if node == "" {
		node = "host://local"
	}

	// 선행 정찰 — 이 머신에 실제로 도는 JVM을 먼저 조사한다(openssl의 /proc 스캔과 대칭).
	// 호출자가 JDK 경로를 몰라도 되게, 발견된 JVM의 java 바이너리를 기본값으로 삼는다.
	jvms, st := jvm.ScanJVMs()
	for _, j := range jvms {
		via := "exe"
		if j.ViaLibjvm {
			via = "libjvm"
		}
		fmt.Fprintf(os.Stderr, "[jvmscan] 발견 JVM pid=%d ver=%s home=%s (%s)\n",
			j.PID, nz(j.Version), nz(j.JavaHome), via)
	}
	if st.ProcUnavailable {
		// "JVM 0개"와 "관측하지 못했다"를 같은 얼굴로 내보내지 않는다(§2.6).
		fmt.Fprintln(os.Stderr, "[jvmscan] ⚠ /proc를 열 수 없어 프로세스를 열거하지 못했다 — JVM이 없는 것이 아니라 관측하지 못한 것이다(리눅스에서 실행할 것)")
	}
	fmt.Fprintf(os.Stderr, "[jvmscan] 정찰: 접근 %d · 불가 %d(관측하지 못함) · JVM %d\n", st.Accessible, st.Denied, st.WithJVM)

	// --pid: 그 JVM 하나만. 정찰에 없으면 조용히 전부 훑지 않고 실패한다 — 사용자가 지목한
	// 대상을 관측하지 못한 것은 갭이지 "전부 보기"로 갈아탈 이유가 아니다(§2.5).
	if *pidOnly > 0 {
		var only []jvm.JVMProc
		for _, j := range jvms {
			if j.PID == *pidOnly {
				only = append(only, j)
			}
		}
		if len(only) == 0 {
			fmt.Fprintf(os.Stderr, "[jvmscan] pid=%d 를 실행 중 JVM에서 찾지 못했다(권한·종료 확인)\n", *pidOnly)
			os.Exit(1)
		}
		jvms = only
	}

	// 실 attach 경로 — PQCOTA_JVM_AGENT(collector JAR)가 있으면 발견된 각 PID에 실제 attach해
	// 그 JVM의 provider 체인(동적 등록 포함)을 관측한다. 정찰→attach 대칭 완성.
	// 없으면 프로브 경로(정적 등록 체인, 데모 경량)로 폴백한다.
	if agent := os.Getenv("PQCOTA_JVM_AGENT"); agent != "" && len(jvms) > 0 {
		emit(*out, node, attachAll(node, jvms, agent), true)
		return
	}

	// --pid인데 에이전트가 없으면 프로브 경로(머신 전역 정적 체인)로 새지 않는다. 그 PID의
	// java.security를 Go가 직접 읽는다 — 동적 등록은 사각이라 갭으로 고지된다.
	if *pidOnly > 0 {
		j := jvms[0]
		c, err := jvm.StaticFallbackGo(j.PID, j.JavaHome)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[jvmscan] pid=%d 정적 폴백 실패: %v\n", j.PID, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[jvmscan] 정적 폴백 pid=%d: provider %d개 (동적 등록은 사각 — 관측하지 못함)\n",
			j.PID, len(c.Providers))
		emit(*out, node, []*discoveryv1.CollectionResult{jvm.BuildResultFor(node, c, j.Ident())}, false)
		return
	}

	// 프로브 경로 — JAVA_BIN 명시가 있으면 그것, 없으면 발견된 JVM의 java, 그것도 없으면 PATH의 java.
	javaBin := os.Getenv("JAVA_BIN")
	if javaBin == "" && len(jvms) > 0 {
		javaBin = jvms[0].JavaBin
	}
	if javaBin == "" {
		javaBin = "java"
	}

	dir, err := os.MkdirTemp("", "jvmscan")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tmp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "Probe.java")
	if err := os.WriteFile(src, []byte(probeSrc), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write probe:", err)
		os.Exit(1)
	}

	var args []string
	if cp := os.Getenv("JVMSCAN_CP"); cp != "" {
		args = append(args, "--class-path", cp)
	}
	args = append(args, src)
	probeOut, err := exec.Command(javaBin, args...).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[jvmscan] java 실행 경고: %v\n%s\n", err, probeOut)
	}

	c := jvm.ParseProviders(string(probeOut))
	emit(*out, node, []*discoveryv1.CollectionResult{jvm.BuildResult(node, c)}, false)
	fmt.Fprintf(os.Stderr, "[jvmscan] %s: provider %d개\n", node, len(c.Providers))
}

// emit — 수집 결과를 요청한 형식으로 낸다. jsonl=true면 한 줄에 하나(여러 JVM), 아니면
// 들여쓴 단일 객체.
func emit(mode, node string, results []*discoveryv1.CollectionResult, jsonl bool) {
	if mode == "json" {
		enc := protojson.MarshalOptions{Multiline: !jsonl, Indent: "  "}
		for _, r := range results {
			b, err := enc.Marshal(r)
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal:", err)
				continue
			}
			os.Stdout.Write(b)
			if jsonl {
				os.Stdout.Write([]byte("\n"))
			}
		}
	}
	if mode == "table" {
		w := os.Stdout
		tbl, err := localview.Render(node, results)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[jvmscan]", err)
			os.Exit(1)
		}
		fmt.Fprintf(w, "== pqcota JVM Discovery — %s (읽기전용·저장 안 함) ==\n", node)
		fmt.Fprintf(w, "관측한 JVM %d개\n\n", len(results))
		fmt.Fprint(w, tbl)
	}
}

// emitRecon — 정찰 결과를 JSON 배열로 stdout에. 관측(attach·프로브)은 하지 않는다.
// 접근 불가 프로세스 수까지 함께 내보내 "관측하지 못한 게 있다"를 오케스트레이터도 알 수 있게 한다(§2.6).
func emitRecon() {
	jvms, st := jvm.ScanJVMs()
	type reconJVM struct {
		PID           int    `json:"pid"`
		App           string `json:"app,omitempty"`
		JavaHome      string `json:"javaHome,omitempty"`
		Version       string `json:"version,omitempty"`
		AttachCapable bool   `json:"attachCapable"`
	}
	out := make([]reconJVM, 0, len(jvms))
	for _, j := range jvms {
		out = append(out, reconJVM{PID: j.PID, App: j.App, JavaHome: j.JavaHome,
			Version: j.Version, AttachCapable: j.AttachCapable})
	}
	b, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "recon marshal:", err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
	os.Stdout.Write([]byte("\n"))
	fmt.Fprintf(os.Stderr, "[jvmscan -recon] JVM %d개 · 접근 %d · 불가 %d(관측하지 못함)\n", len(out), st.Accessible, st.Denied)
}

// nativeOutPath — 에이전트가 **대상 안에서** 쓸 출력 경로. 대상의 /tmp 기준이라 PID로 갈라
// 여러 JVM이 서로 덮어쓰지 않게 한다(호스트에서 읽을 때는 /proc/<pid>/root/가 앞에 붙는다).
func nativeOutPath(pid int) string { return fmt.Sprintf("/tmp/pqcota-providers-%d.txt", pid) }

func nz(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// attachAll — 발견된 각 JVM에 attach(agent JAR)해 provider 체인을 관측하고 JVM별로 구별되는
// CollectionResult를 모아 돌려준다. attach 실패는 갭으로 센다(조용히 0이 되지 않게 §2.5).
func attachAll(node string, jvms []jvm.JVMProc, agent string) []*discoveryv1.CollectionResult {
	// attach 경로는 3계층이다(collector 배포 설계 §2) — 앞이 막히면 뒤로, 다 막히면 정직히 갭.
	//   ① Go 네이티브: JDK 없이 HotSpot 프로토콜로 직접. JRE·jlink·최소 컨테이너까지 커버.
	//   ② JDK 클라이언트: 벤더 무관(OpenJ9 등 HotSpot 아닌 JVM). 머신에 JDK가 있어야.
	//   ③ (호출부 밖) 정적 폴백 — 동적 등록은 사각으로 남고 갭·강등으로 고지(§2.5·§2.6).
	client := jvm.AttachClient(jvms) // ②용 — 대상이 JRE여도 머신의 다른 JDK를 쓴다
	attach := func(j jvm.JVMProc) (jvm.Collected, error) {
		if c, err := jvm.NativeAttach(j.PID, agent, nativeOutPath(j.PID)); err == nil {
			return c, nil
		} else {
			fmt.Fprintf(os.Stderr, "[jvmscan] 네이티브 attach 실패 pid=%d: %v → JDK 클라이언트로 재시도\n", j.PID, err)
		}
		bin := j.JavaBin
		if !j.AttachCapable && client != "" {
			bin = client
		}
		if bin != "" && (j.AttachCapable || client != "") {
			run := jvm.SubprocessRunner(bin, agent)
			if c, err := run(node, map[string]string{"pid": strconv.Itoa(j.PID)}); err == nil {
				return c, nil
			} else {
				fmt.Fprintf(os.Stderr, "[jvmscan] JDK 클라이언트 attach 실패 pid=%d: %v → 정적 폴백\n", j.PID, err)
			}
		}
		// ③ 정적 폴백 — java.security는 텍스트 파일이라 Go가 직접 읽는다. 어떤 JVM·런타임이어도
		// 최소한 정적 등록 체인은 낸다(강등·갭 고지). 관측 실패가 조용한 0이 되지 않게(§2.5).
		c, err := jvm.StaticFallbackGo(j.PID, j.JavaHome)
		if err != nil {
			return jvm.Collected{}, fmt.Errorf("attach 전 경로 실패, 정적 폴백도 불가: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[jvmscan] 정적 폴백 pid=%d: provider %d개 (동적 등록은 사각 — 관측하지 못함)\n",
			j.PID, len(c.Providers))
		return c, nil
	}
	results, ast := jvm.AttachAll(jvms, attach)
	out := make([]*discoveryv1.CollectionResult, 0, len(results))
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "[jvmscan] attach 실패 pid=%d: %v (갭)\n", r.JVM.PID, r.Err)
			continue
		}
		// 식별자는 앱(main·jar) 우선, 없으면 JAVA_HOME→exe — PID는 휘발이라 이력이 깨진다.
		out = append(out, jvm.BuildResultFor(node, r.Collected, r.JVM.Ident()))
	}
	fmt.Fprintf(os.Stderr, "[jvmscan] attach: 발견 %d · 성공 %d · 실패 %d(갭) → 방출 %d\n",
		ast.Discovered, ast.Attached, ast.Failed, len(out))
	return out
}
