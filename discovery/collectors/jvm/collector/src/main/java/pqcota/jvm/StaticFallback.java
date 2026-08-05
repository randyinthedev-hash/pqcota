package pqcota.jvm;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

/**
 * attach 불가(JEP 451 / DisableAttachMechanism) 시 정적 폴백 (설계 §2.2 열화 경로 판정).
 * java.security의 <b>정적 등록</b> provider만 읽고, runtime-introspection 갭을 명시한다.
 * 동적 등록(addProvider)은 이 경로에서 사각지대이며 그 사실 자체를 갭으로 보고한다(§2.6).
 * evidence_strength는 confirmed → inferred로 하향.
 */
final class StaticFallback {
    static void run(String javaHome, String out) {
        List<String> providers = new ArrayList<>();
        Path f = Path.of(javaHome, "conf", "security", "java.security");
        try {
            for (String line : Files.readAllLines(f)) {
                String t = line.trim();
                if (t.startsWith("security.provider.") && t.contains("=")) {
                    String v = t.substring(t.indexOf('=') + 1).trim();
                    if (!v.isEmpty()) providers.add(v);
                }
            }
        } catch (IOException e) {
            // 파일이 없거나 못 읽어도 폴백 산출은 낸다 — 빈 provider + 갭 명시(§2.6).
        }
        StringBuilder sb = new StringBuilder("PQCOTA_STATIC_FALLBACK_BEGIN\n");
        sb.append("evidence_strength=inferred\n");
        sb.append("gap=runtime-introspection\n"); // 동적 등록 사각지대 명시
        for (int i = 0; i < providers.size(); i++) {
            sb.append(i).append('|').append(providers.get(i)).append('\n');
        }
        sb.append("PQCOTA_STATIC_FALLBACK_END\n");
        try {
            Files.writeString(Path.of(out), sb.toString());
        } catch (IOException e) {
            System.err.println("[fallback] 출력 실패: " + e.getMessage());
        }
    }

    private StaticFallback() {}
}
