package jvm

import (
	"path/filepath"
	"strings"
)

// JVM 정찰 — "머신에 무엇이 실제로 도는가"를 collector가 직접 조사한다.
//
// openssl은 ScanHost가 /proc를 훑어 로드된 libssl을 스스로 찾는데, jvm은 그동안 호출자가
// PID·JDK 경로를 미리 알아 넘겨야 했다(비대칭). ScanJVMs가 그 정찰을 맡아 대칭을 맞춘다.
// 여기(파싱)는 순수 함수 — /proc I/O는 procscan.go(linux)에 분리해 테스트 가능하게 둔다.

// JVMProc — 머신에서 발견된 실행 중 JVM 하나.
type JVMProc struct {
	PID       int
	Exe       string // /proc/<pid>/exe 해소 결과(대개 런처 java 경로)
	App       string // cmdline에서 뽑은 실행 앱(main 클래스 또는 -jar) — 자산 식별의 안정 키
	JavaHome  string // exe/libjvm 경로에서 파생(best-effort, 못 짚으면 "")
	JavaBin   string // <JavaHome>/bin/java, 없으면 Exe
	Version   string // <JavaHome>/release의 JAVA_VERSION(best-effort, 없으면 "")
	ViaLibjvm bool   // exe 이름이 아니라 maps의 libjvm.so로 식별됨
	// AttachCapable — 이 JVM의 JAVA_HOME이 jdk.attach 있는 **JDK**인가(순수 JRE면 false).
	// ★ 이게 false여도 attach를 포기하지 않는다 — Go 네이티브 경로(①)는 JDK 없이 붙는다.
	// 이 값은 **② JDK 클라이언트 폴백의 후보 선택**(OpenJ9 등)과 실패 사유 설명에 쓰인다.
	AttachCapable bool
}

// Ident — 이 JVM을 자산으로 구별하는 안정 식별자. 앱(main·jar)이 있으면 그것 —
// 한 JDK에 앱이 여럿이어도 구별되고, 재시작해도 같다(휘발 PID과 달리). 없으면 JAVA_HOME→Exe.
func (j JVMProc) Ident() string {
	switch {
	case j.App != "":
		return j.App
	case j.JavaHome != "":
		return j.JavaHome
	default:
		return j.Exe
	}
}

// JVMScanStats — 스캔 커버리지(완전성 맵 원천, openssl ScanStats와 대칭).
type JVMScanStats struct {
	Accessible int // /proc 접근 가능 프로세스
	Denied     int // 접근 불가(타 사용자·종료) — 갭(≠부재, §2.6)
	WithJVM    int // JVM으로 식별된 프로세스

	// ProcUnavailable — `/proc`를 열 수 없었나(마운트 안 된 컨테이너·chroot·비-리눅스).
	// 이때 "JVM 0개"는 **없다가 아니라 관측하지 못했다**이다. 구별하지 않으면 결함이 갭으로 위장된다(§2.6).
	ProcUnavailable bool
}

func isJavaExe(exe string) bool { return filepath.Base(exe) == "java" }

// AttachLibRel — attach 능력의 표식. jdk.attach 모듈의 네이티브 라이브러리로, **JDK엔 있고 순수
// JRE엔 없다**. 프로세스를 띄우지 않고 파일 존재만으로 판별한다(정찰은 가벼워야 한다).
const AttachLibRel = "lib/libattach.so"

// attachCapable — 이 JAVA_HOME으로 attach를 걸 수 있나(= jdk.attach 있는 JDK인가).
// exists를 주입받아 실물 파일시스템 없이 테스트된다.
//
// 왜 필요한가 — ① **② JDK 클라이언트 폴백**(비-HotSpot용)의 후보를 고르는 데 쓰이고,
// ② attach 실패 사유를 겪기 전에 미리 설명해 준다(§2.6 갭 고지의 질).
// ★ false라고 attach를 포기하는 뜻은 아니다 — 1순위 Go 네이티브는 JDK 없이 붙는다.
func attachCapable(javaHome string, exists func(string) bool) bool {
	if javaHome == "" {
		return false // JAVA_HOME을 못 짚었으면 모른다 — 가능하다고 단정하지 않는다(§2.5)
	}
	return exists(filepath.Join(javaHome, AttachLibRel))
}

// deriveJavaHome — 런처 exe 또는 libjvm.so 경로에서 JAVA_HOME을 추정한다(best-effort).
// 못 짚으면 "" — 추측해 채우지 않는다(§2.5).
func deriveJavaHome(exe, libjvm string) string {
	if exe != "" {
		if d := filepath.Dir(exe); filepath.Base(d) == "bin" { // .../bin/java → .../
			return filepath.Dir(d)
		}
	}
	if libjvm != "" {
		if i := strings.Index(libjvm, "/lib/"); i > 0 { // .../lib[/<arch>]/server/libjvm.so
			return libjvm[:i]
		}
	}
	return ""
}

func javaBinFor(home, exe string) string {
	if home != "" {
		return filepath.Join(home, "bin", "java")
	}
	return exe
}

// parseMainId — cmdline(NUL 구분)에서 이 JVM이 도는 **앱**을 식별한다(main 클래스 또는 -jar).
// 앱은 재시작해도 같아(휘발 PID과 달리) 이력의 안정 식별자로 맞다. 못 짚으면 "".
//
// java 인자 문법을 최소 커버: -jar <x> → x 파일명, -m/--module <mod/Main> → 그대로,
// -cp/-classpath/--class-path/-p/--module-path <v> → 값 건너뜀, 그 외 -플래그(-Xmx·-Dk=v·-XX:.. 단일
// 토큰) → 건너뜀, 첫 비플래그 → main 클래스.
func parseMainId(cmdline string) string {
	args := strings.Split(strings.Trim(cmdline, "\x00"), "\x00")
	for i := 1; i < len(args); i++ { // argv[0]=java 런처 건너뜀
		a := args[i]
		switch a {
		case "-jar":
			if i+1 < len(args) {
				return filepath.Base(args[i+1])
			}
			return ""
		case "-m", "--module":
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		case "-cp", "-classpath", "--class-path", "-p", "--module-path":
			i++ // 값 건너뜀
			continue
		}
		if strings.HasPrefix(a, "-") || a == "" {
			continue
		}
		return a // 첫 비플래그 = main 클래스
	}
	return ""
}

// parseReleaseVersion — JDK의 release 파일에서 JAVA_VERSION을 뽑는다. 없으면 "".
func parseReleaseVersion(release string) string {
	for _, line := range strings.Split(release, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "JAVA_VERSION="); ok {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}
