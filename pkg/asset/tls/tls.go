package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"
)

const (
	keySize = 2048
)

// CertCfg contains all needed fields to configure a new certificate
type CertCfg struct {
	DNSNames     []string
	ExtKeyUsages []x509.ExtKeyUsage
	IPAddresses  []net.IP
	KeyUsages    x509.KeyUsage
	Subject      pkix.Name
	Validity     time.Duration
	IsCA         bool
}

// rsaPublicKey reflects the ASN.1 structure of a PKCS#1 public key.
type rsaPublicKey struct {
	N *big.Int
	E int
}

// generatePrivateKey generates an RSA Private key and returns the value
func generatePrivateKey() (*rsa.PrivateKey, error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, fmt.Errorf("error generating RSA private key: %v", err)
	}

	return rsaKey, nil
}

// selfSignedCACert Creates a self signed CA certificate
func selfSignedCACert(cfg *CertCfg, key *rsa.PrivateKey) (*x509.Certificate, error) {
	var err error

	cert := x509.Certificate{
		BasicConstraintsValid: true,
		IsCA:         cfg.IsCA,
		KeyUsage:     cfg.KeyUsages,
		NotAfter:     time.Now().Add(cfg.Validity),
		NotBefore:    time.Now(),
		SerialNumber: new(big.Int).SetInt64(0),
		Subject:      cfg.Subject,
	}
	// verifies that the CN and/or OU for the cert is set
	if len(cfg.Subject.CommonName) == 0 || len(cfg.Subject.OrganizationalUnit) == 0 {
		return nil, fmt.Errorf("certification's subject is not set, or invalid")
	}
	pub := key.Public()
	cert.SubjectKeyId, err = generateSubjectKeyID(pub)
	if err != nil {
		return nil, fmt.Errorf("failed to set subject key identifier: %v", err)
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &cert, &cert, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("error creating certificate: %v", err)
	}
	return x509.ParseCertificate(certBytes)
}

// signedCertificate creates a new X.509 certificate based on a template.
func signedCertificate(
	cfg *CertCfg,
	csr *x509.CertificateRequest,
	key *rsa.PrivateKey,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey,
) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	certTmpl := x509.Certificate{
		DNSNames:              csr.DNSNames,
		ExtKeyUsage:           cfg.ExtKeyUsages,
		IPAddresses:           csr.IPAddresses,
		KeyUsage:              cfg.KeyUsages,
		NotAfter:              time.Now().Add(cfg.Validity),
		NotBefore:             caCert.NotBefore,
		SerialNumber:          serial,
		Subject:               csr.Subject,
		IsCA:                  cfg.IsCA,
		Version:               3,
		BasicConstraintsValid: true,
	}
	pub := caCert.PublicKey.(*rsa.PublicKey)
	certTmpl.SubjectKeyId, err = generateSubjectKeyID(pub)
	if err != nil {
		return nil, fmt.Errorf("failed to set subject key identifier: %v", err)
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, fmt.Errorf("error creating signed certificate: %v", err)
	}
	return x509.ParseCertificate(certBytes)
}

// generateSubjectKeyID generates a SHA-1 hash of the subject public key.
func generateSubjectKeyID(pub crypto.PublicKey) ([]byte, error) {
	var publicKeyBytes []byte
	var err error

	switch pub := pub.(type) {
	case *rsa.PublicKey:
		publicKeyBytes, err = asn1.Marshal(rsaPublicKey{N: pub.N, E: pub.E})
		if err != nil {
			return nil, err
		}
	case *ecdsa.PublicKey:
		publicKeyBytes = elliptic.Marshal(pub.Curve, pub.X, pub.Y)
	default:
		return nil, errors.New("only RSA and ECDSA public keys supported")
	}

	hash := sha1.Sum(publicKeyBytes)
	return hash[:], nil
}

// generateCert creates a key, csr & a signed cert
// This is useful for apiserver and openshift-apiser cert which will be
// authenticated by the kubeconfig using root-ca.
func generateCert(caKey *rsa.PrivateKey,
	caCert *x509.Certificate,
	cfg *CertCfg) (*rsa.PrivateKey, *x509.Certificate, error) {

	// create a private key
	key, err := generatePrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// create a CSR
	csrTmpl := x509.CertificateRequest{Subject: cfg.Subject, DNSNames: cfg.DNSNames, IPAddresses: cfg.IPAddresses}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &csrTmpl, key)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating certificate request: %v", err)
	}
	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing certificate request: %v", err)
	}

	// create a cert
	cert, err := generateSignedCert(cfg, csr, key, caKey, caCert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a certificate: %v", err)
	}
	return key, cert, nil
}

// generateRootCA creates and returns the root CA
func generateRootCA(key *rsa.PrivateKey, cfg *CertCfg) (*x509.Certificate, error) {
	cert, err := selfSignedCACert(cfg, key)
	if err != nil {
		return nil, fmt.Errorf("error generating self signed certificate: %v", err)
	}
	return cert, nil
}

func generateSignedCert(cfg *CertCfg,
	csr *x509.CertificateRequest,
	key *rsa.PrivateKey,
	caKey *rsa.PrivateKey,
	caCert *x509.Certificate) (*x509.Certificate, error) {
	cert, err := signedCertificate(cfg, csr, key, caCert, caKey)
	if err != nil {
		return nil, fmt.Errorf("error signing certificate: %v", err)
	}
	return cert, nil
}

func generateRootCertKey(cfg *CertCfg) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := generatePrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	crt, err := generateRootCA(key, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a certificate: %v", err)
	}

	return key, crt, nil
}
