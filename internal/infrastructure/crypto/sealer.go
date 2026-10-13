package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"go-api/internal/contract"
	"go-api/internal/domain/port"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// GenerateSealKeys returns a base64 X25519 key pair for JOB_SEAL_PUBLIC_KEY / JOB_SEAL_PRIVATE_KEY.
func GenerateSealKeys() (public, private string, err error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pub[:]), base64.StdEncoding.EncodeToString(priv[:]), nil
}

// GenerateSecretKey returns a base64 32 byte key for SECRETS_ENCRYPTION_KEY.
func GenerateSecretKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func decodeKey(name, value string) (*[32]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("%s must be a base64 32 byte key", name)
	}
	var key [32]byte
	copy(key[:], raw)
	return &key, nil
}

// Sealer encrypts job secrets for the worker-calls public key. The scheduler
// can seal but never open.
type Sealer struct {
	publicKey *[32]byte
}

func NewSealer(base64PublicKey string) (*Sealer, error) {
	key, err := decodeKey("JOB_SEAL_PUBLIC_KEY", base64PublicKey)
	if err != nil {
		return nil, err
	}
	return &Sealer{publicKey: key}, nil
}

var _ port.JobSealer = (*Sealer)(nil)

func (s *Sealer) Seal(secrets map[string]string) (*contract.SealedSecrets, error) {
	if len(secrets) == 0 {
		return nil, nil
	}
	var dataKey [32]byte
	if _, err := rand.Read(dataKey[:]); err != nil {
		return nil, err
	}
	sealedKey, err := box.SealAnonymous(nil, dataKey[:], s.publicKey, rand.Reader)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(secrets))
	for name, value := range secrets {
		var nonce [24]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		values[name] = base64.StdEncoding.EncodeToString(secretbox.Seal(nonce[:], []byte(value), &nonce, &dataKey))
	}
	return &contract.SealedSecrets{
		SealedKey: base64.StdEncoding.EncodeToString(sealedKey),
		Values:    values,
	}, nil
}

// Opener decrypts job secrets in worker-calls.
type Opener struct {
	publicKey  *[32]byte
	privateKey *[32]byte
}

func NewOpener(base64PublicKey, base64PrivateKey string) (*Opener, error) {
	pub, err := decodeKey("JOB_SEAL_PUBLIC_KEY", base64PublicKey)
	if err != nil {
		return nil, err
	}
	priv, err := decodeKey("JOB_SEAL_PRIVATE_KEY", base64PrivateKey)
	if err != nil {
		return nil, err
	}
	return &Opener{publicKey: pub, privateKey: priv}, nil
}

var _ port.JobOpener = (*Opener)(nil)

var errCannotOpen = errors.New("cannot open sealed secrets")

func (o *Opener) Open(sealed *contract.SealedSecrets) (map[string]string, error) {
	if sealed == nil {
		return map[string]string{}, nil
	}
	rawKey, err := base64.StdEncoding.DecodeString(sealed.SealedKey)
	if err != nil {
		return nil, errCannotOpen
	}
	keyBytes, ok := box.OpenAnonymous(nil, rawKey, o.publicKey, o.privateKey)
	if !ok || len(keyBytes) != 32 {
		return nil, errCannotOpen
	}
	var dataKey [32]byte
	copy(dataKey[:], keyBytes)

	out := make(map[string]string, len(sealed.Values))
	for name, encoded := range sealed.Values {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(raw) < 24 {
			return nil, errCannotOpen
		}
		var nonce [24]byte
		copy(nonce[:], raw[:24])
		plain, ok := secretbox.Open(nil, raw[24:], &nonce, &dataKey)
		if !ok {
			return nil, errCannotOpen
		}
		out[name] = string(plain)
	}
	return out, nil
}
