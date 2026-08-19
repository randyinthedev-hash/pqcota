package pqcota.jvm;

import com.sun.tools.attach.VirtualMachine;

/**
 * attach 클라이언트 (jvm-collector). 실행 중 대상 JVM(pid)에 붙어 에이전트 JAR를 loadAgent.
 * jdk.attach 모듈 필요. 전제: 동일 UID(또는 root), attach 미차단(JEP 451 대응은 설계 §2.2).
 *
 * 사용: java --add-modules jdk.attach -cp collector.jar pqcota.jvm.Attacher &lt;pid&gt; &lt;agent.jar&gt; [out]
 *
 * 순수 Java — 이 사이드카에 Kotlin·Gradle이 필요 없다. 불가피한 제약은 "JVM 안에서 실행"뿐이고
 * 플랫폼 자신의 언어(Java)로 충분하다. 빌드는 javac + jar(run.sh).
 */
public final class Attacher {
    public static void main(String[] args) {
        String pid = args[0];
        String agentJar = args[1];
        String out = args.length > 2 ? args[2] : "/tmp/pqcota-providers.txt";
        try {
            VirtualMachine vm = VirtualMachine.attach(pid);
            try {
                vm.loadAgent(agentJar, out);
            } finally {
                vm.detach();
            }
            System.out.println("[attacher] attach succeeded → the real getProviders() (confirmed)");
        } catch (Exception e) {
            // JEP 451 / DisableAttachMechanism / 권한 → 정적 폴백(설계 §2.2, S2-4).
            System.out.println("[attacher] attach unavailable (" + e.getClass().getSimpleName()
                    + ") → static fallback (inferred + gap)");
            StaticFallback.run(System.getProperty("java.home"), out);
        }
    }

    private Attacher() {}
}
