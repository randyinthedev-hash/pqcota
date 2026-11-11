// SPDX-FileCopyrightText: 2026 Great Honor <randyinthedev@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Example for the guide "Moving document-key transport from RSA-OAEP to ML-KEM" (README.md in this directory).
// Run: java -cp bcprov-jdk18on-<version>.jar DocKeyExample.java
//
// Both ways encrypt the document with AES-256-GCM. What changes is how the document key reaches the recipient.
//   Current:    wrap the document key with the recipient's RSA public key (RSA-OAEP).
//   Transition: get a shared secret with ML-KEM → derive a key-encryption key (KEK) with HKDF → wrap the document key
//               with the KEK (AES-KWP).
// ML-KEM encrypts neither the document nor the document key. It is a key-encapsulation mechanism that yields a shared secret.
// A production file format is out of scope: the output here is a bundle of fields for the tests.
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.security.spec.MGF1ParameterSpec;
import java.util.Arrays;
import javax.crypto.*;
import javax.crypto.spec.*;
import org.bouncycastle.jcajce.spec.HKDFParameterSpec;
import org.bouncycastle.jce.provider.BouncyCastleProvider;

public class DocKeyExample {
  static final SecureRandom RNG = new SecureRandom();
  static final byte[] INFO = "pqcota-guide-example doc-key KEK v1".getBytes(StandardCharsets.UTF_8);

  // Document encryption: the same in both ways.
  record Sealed(byte[] iv, byte[] ct) {}
  static Sealed seal(SecretKey docKey, byte[] doc) throws GeneralSecurityException {
    byte[] iv = new byte[12]; RNG.nextBytes(iv);
    Cipher c = Cipher.getInstance("AES/GCM/NoPadding");
    c.init(Cipher.ENCRYPT_MODE, docKey, new GCMParameterSpec(128, iv));
    return new Sealed(iv, c.doFinal(doc));
  }
  static byte[] open(SecretKey docKey, Sealed s) throws GeneralSecurityException {
    Cipher c = Cipher.getInstance("AES/GCM/NoPadding");
    c.init(Cipher.DECRYPT_MODE, docKey, new GCMParameterSpec(128, s.iv()));
    return c.doFinal(s.ct());
  }
  static SecretKey newDocKey() throws GeneralSecurityException {
    KeyGenerator kg = KeyGenerator.getInstance("AES"); kg.init(256); return kg.generateKey();
  }

  // ── Current: wrap the document key with RSA-OAEP ──
  static byte[] wrapRsa(PublicKey recipient, SecretKey docKey) throws GeneralSecurityException {
    Cipher c = Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding");
    c.init(Cipher.WRAP_MODE, recipient, new OAEPParameterSpec("SHA-256", "MGF1", MGF1ParameterSpec.SHA256, PSource.PSpecified.DEFAULT));
    return c.wrap(docKey);
  }
  static SecretKey unwrapRsa(PrivateKey mine, byte[] wrapped) throws GeneralSecurityException {
    Cipher c = Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding");
    c.init(Cipher.UNWRAP_MODE, mine, new OAEPParameterSpec("SHA-256", "MGF1", MGF1ParameterSpec.SHA256, PSource.PSpecified.DEFAULT));
    return (SecretKey) c.unwrap(wrapped, "AES", Cipher.SECRET_KEY);
  }

  // ── Transition: ML-KEM → HKDF → KEK → wrap the document key with AES-KWP ──
  record KemWrapped(byte[] encapsulation, byte[] wrappedKey) {}
  static SecretKey kek(byte[] sharedSecret) throws GeneralSecurityException {
    SecretKeyFactory f = SecretKeyFactory.getInstance("HKDF-SHA256", "BC");
    byte[] k = f.generateSecret(new HKDFParameterSpec(sharedSecret, null, INFO, 32)).getEncoded();
    return new SecretKeySpec(k, "AES");
  }
  static KemWrapped wrapMlKem(PublicKey recipient, SecretKey docKey) throws GeneralSecurityException {
    KEM.Encapsulated e = KEM.getInstance("ML-KEM", "BC").newEncapsulator(recipient).encapsulate();
    Cipher c = Cipher.getInstance("AES/KWP/NoPadding");
    c.init(Cipher.WRAP_MODE, kek(e.key().getEncoded()));
    return new KemWrapped(e.encapsulation(), c.wrap(docKey));
  }
  static SecretKey unwrapMlKem(PrivateKey mine, KemWrapped w) throws GeneralSecurityException {
    SecretKey shared = KEM.getInstance("ML-KEM", "BC").newDecapsulator(mine).decapsulate(w.encapsulation());
    Cipher c = Cipher.getInstance("AES/KWP/NoPadding");
    c.init(Cipher.UNWRAP_MODE, kek(shared.getEncoded()));
    return (SecretKey) c.unwrap(w.wrappedKey(), "AES", Cipher.SECRET_KEY);
  }

  // ── Tests ──
  static int fails = 0;
  static void check(String name, boolean ok) { System.out.println((ok ? "PASS " : "FAIL ") + name); if (!ok) fails++; }
  static boolean refused(ThrowingRun r) { try { r.run(); return false; } catch (GeneralSecurityException e) { return true; } }
  interface ThrowingRun { void run() throws GeneralSecurityException; }
  static byte[] flip(byte[] b, int i) { byte[] c = b.clone(); c[i] ^= 1; return c; }

  public static void main(String[] args) throws Exception {
    Security.addProvider(new BouncyCastleProvider());
    byte[] doc = "pqcota guide example document".getBytes(StandardCharsets.UTF_8);

    KeyPairGenerator rsaGen = KeyPairGenerator.getInstance("RSA"); rsaGen.initialize(3072);
    KeyPair rsa = rsaGen.generateKeyPair();
    SecretKey k1 = newDocKey(); Sealed s1 = seal(k1, doc); byte[] w1 = wrapRsa(rsa.getPublic(), k1);
    check("current (RSA-OAEP): round trip", Arrays.equals(open(unwrapRsa(rsa.getPrivate(), w1), s1), doc));

    KeyPair mlkem = KeyPairGenerator.getInstance("ML-KEM-768", "BC").generateKeyPair();
    SecretKey k2 = newDocKey(); Sealed s2 = seal(k2, doc); KemWrapped w2 = wrapMlKem(mlkem.getPublic(), k2);
    check("transition (ML-KEM-768): round trip", Arrays.equals(open(unwrapMlKem(mlkem.getPrivate(), w2), s2), doc));
    check("transition: a changed encapsulation is refused (other shared secret → unwrap fails)",
        refused(() -> unwrapMlKem(mlkem.getPrivate(), new KemWrapped(flip(w2.encapsulation(), 0), w2.wrappedKey()))));
    check("transition: a changed wrapped document key is refused",
        refused(() -> unwrapMlKem(mlkem.getPrivate(), new KemWrapped(w2.encapsulation(), flip(w2.wrappedKey(), 3)))));
    check("transition: a changed document ciphertext is refused (GCM authentication)",
        refused(() -> open(unwrapMlKem(mlkem.getPrivate(), w2), new Sealed(s2.iv(), flip(s2.ct(), 0)))));
    KeyPair other = KeyPairGenerator.getInstance("ML-KEM-768", "BC").generateKeyPair();
    check("transition: another recipient's private key does not unwrap it", refused(() -> unwrapMlKem(other.getPrivate(), w2)));
    // These tests only show that changing a bundle that was already made is refused. They do not show who sent it:
    // anyone who has the recipient's public key can make a new bundle (README, Limits).

    System.out.printf("sizes: RSA-3072 wrapped key %dB · ML-KEM-768 encapsulation %dB + wrapped key %dB · public key %dB (X.509 SubjectPublicKeyInfo encoding)%n",
        w1.length, w2.encapsulation().length, w2.wrappedKey().length, mlkem.getPublic().getEncoded().length);
    System.out.println("JDK " + System.getProperty("java.version") + " · " + Security.getProvider("BC").getInfo());
    System.exit(fails == 0 ? 0 : 1);
  }
}
