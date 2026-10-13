package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"go-api/internal/infrastructure/config"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// MaxMessageBytes matches the broker and topic max.message.bytes (4 MB).
const MaxMessageBytes = 4 * 1024 * 1024

func baseOpts(cfg config.KafkaConfig) ([]kgo.Opt, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: no brokers configured")
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
	}

	if cfg.TLS {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if cfg.TLSCAFile != "" {
			pem, err := os.ReadFile(cfg.TLSCAFile)
			if err != nil {
				return nil, fmt.Errorf("kafka: read CA file: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("kafka: no certificate found in %s", cfg.TLSCAFile)
			}
			tlsCfg.RootCAs = pool
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	switch strings.ToUpper(cfg.SASLMechanism) {
	case "":
	case "SCRAM-SHA-512":
		auth := scram.Auth{User: cfg.Username, Pass: cfg.Password}
		opts = append(opts, kgo.SASL(auth.AsSha512Mechanism()))
	case "SCRAM-SHA-256":
		auth := scram.Auth{User: cfg.Username, Pass: cfg.Password}
		opts = append(opts, kgo.SASL(auth.AsSha256Mechanism()))
	default:
		return nil, fmt.Errorf("kafka: unsupported SASL mechanism %q", cfg.SASLMechanism)
	}

	return opts, nil
}

func newClient(cfg config.KafkaConfig, extra ...kgo.Opt) (*kgo.Client, error) {
	opts, err := baseOpts(cfg)
	if err != nil {
		return nil, err
	}
	cl, err := kgo.NewClient(append(opts, extra...)...)
	if err != nil {
		return nil, fmt.Errorf("kafka: new client: %w", err)
	}
	return cl, nil
}

// Ping verifies that at least one broker is reachable.
func Ping(ctx context.Context, cfg config.KafkaConfig) error {
	cl, err := newClient(cfg)
	if err != nil {
		return err
	}
	defer cl.Close()
	return cl.Ping(ctx)
}
