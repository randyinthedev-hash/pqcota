// 데모용 Java 크립토 워크로드 — 지정된 JCA provider를 등록하고 주기적으로 서명한다.
// "배포된 Java 앱"을 실제 실행 상태로 두어, pqcota-jvmscan이 JCA provider 체인을 관측하게 한다.
//
// 등록할 provider는 환경변수 PQCOTA_PROVIDERS(콤마 목록)로 받는다 — 미설정이면 BC(기존 데모 호환).
// 커스텀 토폴로지가 노드별로 provider를 달리해 등급 차이(BC 유무)를 보이게 한다.
import java.security.*;
import org.bouncycastle.jce.provider.BouncyCastleProvider;

public class CryptoApp {
  public static void main(String[] args) throws Exception {
    String spec = System.getenv("PQCOTA_PROVIDERS");
    if (spec == null) spec = "BC"; // 미설정 = 기존 데모대로 BC 등록
    for (String p : spec.split(",")) {
      p = p.trim();
      if (p.equals("BC")) {
        Security.addProvider(new BouncyCastleProvider());
        System.out.println("[java-app] BouncyCastle 런타임 등록");
      } else if (!p.isEmpty()) {
        System.out.println("[java-app] provider " + p + " 는 데모 미지원 — 건너뜀");
      }
    }
    System.out.println("[java-app] 등록 완료 — providers=" + Security.getProviders().length);
    KeyPairGenerator kpg = KeyPairGenerator.getInstance("EC");
    while (true) {
      KeyPair kp = kpg.generateKeyPair();
      Signature sig = Signature.getInstance("SHA256withECDSA");
      sig.initSign(kp.getPrivate());
      sig.update("pqcota-demo".getBytes());
      sig.sign();
      Thread.sleep(5000);
    }
  }
}
