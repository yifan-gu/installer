package tls

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"

	"github.com/openshift/installer/pkg/asset"
)

// RootCA contains the private key and the cert that's
// self-signed as the root CA.
type RootCA struct {
	rootDir string
}

var _ asset.Asset = (*CertKey)(nil)

func (c *RootCA) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

func (c *RootCA) Generate(parents map[asset.Asset]*asset.State) (*asset.State, error) {
	cfg := &CertCfg{
		Subject:   pkix.Name{CommonName: "root-ca", OrganizationalUnit: []string{"openshift"}},
		KeyUsages: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		Validity:  validityTenYears,
		IsCA:      true,
	}

	key, crt, err := generateRootCertKey(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RootCA %v", err)
	}

	var st asset.State
	st.Contents = []asset.Content{
		{
			Name: assetFilePath(c.rootDir, "root-ca.key"),
			Data: []byte(PrivateKeyToPem(key)),
		},
		{
			Name: assetFilePath(c.rootDir, "root-ca.crt"),
			Data: []byte(CertToPem(crt)),
		},
	}

	if err := WriteContents(&st); err != nil {
		return nil, err
	}

	return &st, nil
}
