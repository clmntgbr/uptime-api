package crypto

import "testing"

func TestAESCipher_RoundTrip(t *testing.T) {
	key, err := GenerateSecretKey()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewAESCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt("s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	if enc == "s3cr3t" {
		t.Fatal("value not encrypted")
	}
	dec, err := c.Decrypt(enc)
	if err != nil || dec != "s3cr3t" {
		t.Fatalf("got %q, %v", dec, err)
	}
	if _, err := c.Decrypt(enc[:len(enc)-2] + "AA"); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestSealer_OnlyPrivateKeyOpens(t *testing.T) {
	pub, priv, err := GenerateSealKeys()
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := NewSealer(pub)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealer.Seal(map[string]string{"api_key": "abc", "password": "p"})
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Values["api_key"] == "abc" {
		t.Fatal("value sent in clear")
	}

	opener, err := NewOpener(pub, priv)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := opener.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if opened["api_key"] != "abc" || opened["password"] != "p" {
		t.Fatalf("unexpected secrets %v", opened)
	}

	otherPub, otherPriv, _ := GenerateSealKeys()
	other, _ := NewOpener(otherPub, otherPriv)
	if _, err := other.Open(sealed); err == nil {
		t.Fatal("another key opened the secrets")
	}
}
