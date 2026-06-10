package auth

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "test-encryption-passphrase"
	plain := "cg-a1b2c3d4-3f9ace0011223344556677889900aabbccddeeff"
	ct, err := Encrypt(plain, secret)
	if err != nil {
		t.Fatal(err)
	}
	if ct == plain {
		t.Fatal("密文不应等于明文")
	}
	got, err := Decrypt(ct, secret)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("解密结果 = %q, 期望 %q", got, plain)
	}
}

func TestEncryptNonceRandomized(t *testing.T) {
	// 相同明文两次加密应得到不同密文（nonce 随机）
	a, _ := Encrypt("same", "k")
	b, _ := Encrypt("same", "k")
	if a == b {
		t.Fatal("两次加密应因随机 nonce 而不同")
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	ct, _ := Encrypt("secret-value", "right-key")
	if _, err := Decrypt(ct, "wrong-key"); err == nil {
		t.Fatal("错误口令应解密失败")
	}
}

func TestDecryptTamperFails(t *testing.T) {
	ct, _ := Encrypt("secret-value", "k")
	// 篡改最后一个字符
	tampered := ct[:len(ct)-1]
	if tampered[len(tampered)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}
	if _, err := Decrypt(tampered, "k"); err == nil {
		t.Fatal("被篡改的密文应解密失败")
	}
}

func TestEncryptEmptySecret(t *testing.T) {
	if _, err := Encrypt("x", ""); err == nil {
		t.Fatal("空口令应返回错误")
	}
	if _, err := Decrypt("x", ""); err == nil {
		t.Fatal("空口令应返回错误")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	if _, err := Decrypt("!!!not-base64!!!", "k"); err == nil {
		t.Fatal("非法 base64 应返回错误")
	}
}
