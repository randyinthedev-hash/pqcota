package procs

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Attribution — 엣지 하나를 앱에 귀속시킨 결과.
//
// **Key가 비었다는 것은 "앱이 없다"가 아니라 "귀속하지 못했다"이다.** 왜 못 했는지는 Reason이
// 말한다 — 관측 갭을 "없음"으로 적지 않는 것과 같은 규칙이다(§2.6).
type Attribution struct {
	Key    string // app_key. 못 잡았으면 빈 값
	Kind   string // "systemd-unit" | "exe-path". Key가 비면 함께 빈다
	Reason string // Key가 빈 이유. 잡았으면 빈 값
}

// 귀속하지 못한 이유 — 사유가 다르면 대응이 다르다.
const (
	// ReasonSocketGone — /proc을 읽는 시점에 그 소켓이 이미 없다. 짧게 붙었다 끊긴 연결이다.
	// 구현으로 창을 좁힐 수는 있어도 없앨 수 없다.
	ReasonSocketGone = "캡처와 조회 사이에 소켓이 닫혔다 — 짧은 연결은 놓친다"
	// ReasonNoPermission — 소켓은 찾았는데 그것을 쥔 프로세스의 fd를 읽을 권한이 없다.
	// CAP_NET_RAW로는 부족하다.
	ReasonNoPermission = "소켓을 쥔 프로세스를 읽을 권한이 없다 — 남의 /proc/PID/fd는 못 읽는다"
	// ReasonAmbiguous — 같은 상대와 통신하는 소켓이 여럿이고 서로 다른 앱이다.
	// **기계가 하나를 고르지 않는다** — 틀린 앱에 귀속하면 조치 대상이 바뀐다.
	ReasonAmbiguous = "같은 상대로 여러 앱이 통신 중이다 — 기계가 하나를 고르지 않는다"
	// ReasonNoAppKey — 프로세스는 찾았는데 안정 키를 뽑지 못했다(cgroup·exe 둘 다 실패).
	ReasonNoAppKey = "프로세스는 찾았으나 안정 키를 뽑지 못했다"
)

// AttributeRemote — 이 호스트에서 remote(ip:port)로 나간 연결을 연 앱을 찾는다.
//
// 흐름: /proc/net/tcp·tcp6에서 그 상대와 맺은 소켓 inode를 찾고 → /proc/*/fd에서 그 inode를 쥔
// PID들을 찾고 → 그중 **부모 사슬에서 가장 얕은 것**을 골라 [AppKey]로 키를 뽑는다.
//
// 가장 얕은 것을 고르는 이유: fd는 상속되므로 부모가 연 연결을 자식들이 그대로 들고 있다.
// 먼저 찾은 PID를 쓰면 연결을 연 쪽이 아니라 그것을 물려받은 쪽에 귀속된다.
func AttributeRemote(procRoot, remoteIP string, remotePort uint32) Attribution {
	inodes, err := socketsTo(procRoot, remoteIP, remotePort)
	if err != nil || len(inodes) == 0 {
		return Attribution{Reason: ReasonSocketGone}
	}
	owners, permDenied := inodeOwners(procRoot, inodes)
	if len(owners) == 0 {
		if permDenied {
			return Attribution{Reason: ReasonNoPermission}
		}
		return Attribution{Reason: ReasonSocketGone}
	}

	// inode마다 소켓을 연 쪽(가장 얕은 PID)을 고르고, 그 키를 모은다.
	keys := map[string]string{} // key → kind
	for _, pids := range owners {
		pid := shallowest(procRoot, pids)
		if k, kind := AppKey(procRoot, pid); k != "" {
			keys[k] = kind
		}
	}
	switch len(keys) {
	case 0:
		return Attribution{Reason: ReasonNoAppKey}
	case 1:
		for k, kind := range keys {
			return Attribution{Key: k, Kind: kind}
		}
	}
	// 여럿이면 고르지 않는다 — 틀린 앱에 귀속하는 것이 비워 두는 것보다 나쁘다.
	return Attribution{Reason: ReasonAmbiguous}
}

// socketsTo — /proc/net/tcp·tcp6에서 remote와 맺은 소켓의 inode들.
func socketsTo(procRoot, remoteIP string, remotePort uint32) ([]uint64, error) {
	want, err := hexAddr(remoteIP, remotePort)
	if err != nil {
		return nil, err
	}
	var out []uint64
	for _, name := range []string{"net/tcp", "net/tcp6"} {
		f, err := os.Open(filepath.Join(procRoot, name))
		if err != nil {
			continue // tcp6가 없는 커널 구성이 있다
		}
		sc := bufio.NewScanner(f)
		sc.Scan() // 헤더
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 10 {
				continue
			}
			if !strings.EqualFold(fields[2], want) {
				continue
			}
			if n, err := strconv.ParseUint(fields[9], 10, 64); err == nil && n != 0 {
				out = append(out, n)
			}
		}
		f.Close()
	}
	return out, nil
}

// hexAddr — "ip:port"를 /proc/net/tcp의 표기(리틀엔디언 hex)로 바꾼다.
func hexAddr(ip string, port uint32) (string, error) {
	b, err := parseIP(ip)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	// /proc/net/tcp는 4바이트 워드마다 호스트 바이트 순서로 적는다.
	for i := 0; i < len(b); i += 4 {
		for j := 3; j >= 0; j-- {
			fmt.Fprintf(&sb, "%02X", b[i+j])
		}
	}
	fmt.Fprintf(&sb, ":%04X", port)
	return sb.String(), nil
}

// inodeOwners — inode → 그것을 쥔 PID들. 두 번째 반환값은 권한 때문에 못 본 프로세스가 있었는지.
func inodeOwners(procRoot string, inodes []uint64) (map[uint64][]int, bool) {
	want := map[string]uint64{}
	for _, in := range inodes {
		want["socket:["+strconv.FormatUint(in, 10)+"]"] = in
	}
	out := map[uint64][]int{}
	denied := false

	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return out, false
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join(procRoot, e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			if os.IsPermission(err) {
				denied = true
			}
			continue
		}
		for _, fd := range fds {
			tgt, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if in, ok := want[tgt]; ok {
				out[in] = append(out[in], pid)
			}
		}
	}
	return out, denied
}

// shallowest — 같은 소켓을 쥔 PID 집합에서 **부모 사슬의 가장 위**를 고른다.
// 집합 안에 조상이 있으면 그쪽이 그 소켓을 연 쪽이다. 뿌리가 여럿이면(예: SCM_RIGHTS로
// 남남끼리 소켓을 주고받은 경우) 결정론을 위해 가장 작은 PID를 고른다.
func shallowest(procRoot string, pids []int) int {
	in := map[int]bool{}
	for _, p := range pids {
		in[p] = true
	}
	roots := map[int]bool{}
	for _, p := range pids {
		cur := p
		for hops := 0; hops < 64; hops++ { // 사슬이 집합 밖으로 나갈 때까지
			pp := ppid(procRoot, cur)
			if pp <= 0 || !in[pp] {
				break
			}
			cur = pp
		}
		roots[cur] = true
	}
	best := 0
	for r := range roots {
		if best == 0 || r < best {
			best = r
		}
	}
	return best
}

// parseIP — "1.2.3.4" 또는 IPv6 문자열을 바이트로. IPv4는 4바이트로 줄인다
// (/proc/net/tcp가 4바이트로 적기 때문 — 16바이트로 넘기면 tcp6 표기와 섞인다).
func parseIP(s string) ([]byte, error) {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, fmt.Errorf("IP가 아니다: %q", s)
	}
	if v4 := ip.To4(); v4 != nil {
		return v4, nil
	}
	return ip.To16(), nil
}

// ppid — /proc/PID/status의 PPid. 못 읽으면 0.
func ppid(procRoot string, pid int) int {
	b, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "status"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "PPid:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(rest))
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}
