#!/usr/bin/env bash
# 생성한 java.security 조각이 **정말로 provider를 등록하는가**를 실제 JVM에서 확인한다.
#
# 빈 파일로 플레이북을 돌리면 "Ansible이 파일을 복사했다"까지만 확인된다. 그건 생성물이 의도한
# 일을 하는지에 대해 아무 말도 하지 않는다 — 조각이 적용조차 되지 않는 경우가 실제로 있다
# (레거시 OpenSSL에서 그랬다). 그래서 여기서 진짜 JAR과 진짜 JVM으로 끝까지 본다.
#
#   ./verify-registration.sh                       기본: jca-provider-inject-bc 케이스
#   ./verify-registration.sh jca-fips-bcfips       다른 케이스
#
# 전제: ./fetch-example-provider.sh 로 BC.jar을 먼저 받아둘 것. JDK는 있으면 쓰고, 없으면 Docker.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
CASE="${1:-jca-provider-inject-bc}"
JAR="$HERE/BC.jar"

[ -f "$JAR" ] || {
	echo "✗ $JAR 없음 — 먼저 ./fetch-example-provider.sh" >&2
	exit 1
}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# 1) 계획 → java.security 조각. 플레이북이 노드에 놓는 바로 그 내용을 꺼낸다.
cd "$ROOT"
go run ./provisioning/cmd/pqcota-provision --level l2 \
	"examples/provisioning/plans/$CASE.json" 2>/dev/null |
	sed -n '/content: |/,/^    - name:/p' | sed -n 's/^          //p' >"$work/pqcota.security"
[ -s "$work/pqcota.security" ] || {
	echo "✗ $CASE 에서 java.security 조각을 얻지 못했다(이 케이스는 config를 배치하지 않는가?)" >&2
	exit 1
}
cp "$JAR" "$work/BC.jar"

# 2) 조각을 얹은 JVM에서 provider 체인을 실제로 읽는다.
#    -Djava.security.properties=<파일>(등호 하나)은 JDK의 java.security에 **덮어쓰기 병합**한다 —
#    노드에서 조각을 병합해 적용하는 것과 같은 효과라, 시스템 파일을 건드리지 않고 확인할 수 있다.
cat >"$work/Show.java" <<'EOF'
import java.security.*;
public class Show {
  public static void main(String[] a) throws Exception {
    System.out.println("── 등록된 provider 순서(조각 적용 후) ──");
    int i = 1;
    for (Provider p : Security.getProviders()) System.out.printf("  %d. %s %s%n", i++, p.getName(), p.getVersionStr());
    System.out.println("── 목표 알고리즘이 실제로 제공되는가 ──");
    for (String alg : new String[]{"ML-KEM", "ML-KEM-768", "ML-DSA"}) {
      String where = null;
      for (Provider p : Security.getProviders())
        for (Provider.Service s : p.getServices())
          if (s.getAlgorithm().equalsIgnoreCase(alg)) { where = p.getName() + " (" + s.getType() + ")"; break; }
      System.out.printf("  %-12s %s%n", alg, where == null ? "없음" : "→ " + where);
    }
    System.out.println("── TLS 협상 그룹 ──");
    System.out.println("  jdk.tls.namedGroups = " + Security.getProperty("jdk.tls.namedGroups"));
  }
}
EOF

# 조각 없이도 한 번 돌린다 — **무엇이 달라졌는가**는 대조해야만 보인다.
: >"$work/empty.security"

if command -v java >/dev/null 2>&1; then
	jvm() { java "$@"; }
	FRAG="$work/pqcota.security"; NONE="$work/empty.security"; JARP="$work/BC.jar"; SRC="$work/Show.java"
elif command -v docker >/dev/null 2>&1; then
	echo "(로컬 JDK 없음 → Docker의 eclipse-temurin:21-jdk 사용)"
	jvm() { docker run --rm -v "$work:/w" -w /w eclipse-temurin:21-jdk java "$@"; }
	FRAG=/w/pqcota.security; NONE=/w/empty.security; JARP=/w/BC.jar; SRC=/w/Show.java
else
	echo "✗ java도 docker도 없다 — 둘 중 하나가 필요하다." >&2
	exit 1
fi

echo "▶ 케이스 $CASE — 조각을 얹고 JVM에서 확인"
echo
jvm -Djava.security.properties="$FRAG" -cp "$JARP" "$SRC" | tee "$work/after.txt"

# ★ 자리 대체를 눈으로 — security.provider.N은 끼워 넣지 않고 그 자리를 차지한다.
jvm -Djava.security.properties="$NONE" -cp "$JARP" "$SRC" >"$work/before.txt" 2>/dev/null || true
echo
echo "── 조각이 provider 목록에 한 일 (조각 없음 → 조각 적용) ──"
# diff는 차이가 있으면 1로 끝난다 — pipefail이 켜져 있어 그 값이 파이프라인 결과가 된다.
# 그래서 결과를 먼저 담고 나서 판단한다(그러지 않으면 차이가 있는데도 "차이 없음"이 함께 찍힌다).
changed=$(diff <(sed -n '/등록된 provider/,/목표 알고리즘/p' "$work/before.txt") \
	<(sed -n '/등록된 provider/,/목표 알고리즘/p' "$work/after.txt") |
	sed 's/^</  빠짐:/; s/^>/  들어옴:/' | grep -E '빠짐|들어옴' || true)
[ -n "$changed" ] && echo "$changed" || echo "  (차이 없음)"
echo
echo '※ security.provider.2는 **끼워 넣지 않고 그 자리를 대체한다** — 위 대조에서 원래 2번이던'
echo "   provider가 빠지는 것이 보인다. 그 provider가 담당하던 서비스(JDK 기본이면 RSA)는 새"
echo "   provider 구현으로 넘어간다. 이것이 전역 변경의 영향 반경(§4.4)이다."
