package tls

import (
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/types"
)

const (
	validityTenYears      = time.Hour * 24 * 365 * 10
	validityThirtyMinutes = time.Minute * 30

	tlsDir = "tls"
)

func assetFilePath(rootDir, filename string) string {
	return filepath.Join(rootDir, tlsDir, filename)
}

func getBaseAddress(cfg *types.InstallConfig) string {
	return fmt.Sprintf("%s.%s", cfg.Name, cfg.BaseDomain)
}

func cidrhost(network net.IPNet, hostNum int) (string, error) {
	ip, err := cidr.Host(&network, hostNum)
	if err != nil {
		return "", err
	}

	return ip.String(), nil
}

func genSubjectForIngressCertKey(cfg *types.InstallConfig) (pkix.Name, error) {
	return pkix.Name{CommonName: getBaseAddress(cfg), Organization: []string{"ingress"}}, nil
}

func genDNSNamesForIngressCertKey(cfg *types.InstallConfig) ([]string, error) {
	baseAddress := getBaseAddress(cfg)
	return []string{
		baseAddress,
		fmt.Sprintf("*.%s", baseAddress),
	}, nil
}

func genDNSNamesForAPIServerCertKey(cfg *types.InstallConfig) ([]string, error) {
	return []string{
		fmt.Sprintf("%s-api.%s", cfg.Name, cfg.BaseDomain),
		"kubernetes", "kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
	}, nil
}

func genIPAddressesForAPIServerCertKey(cfg *types.InstallConfig) ([]net.IP, error) {
	apiServerAddress, err := cidrhost(cfg.Networking.ServiceCIDR, 1)
	if err != nil {
		return nil, err
	}
	return []net.IP{net.ParseIP(apiServerAddress)}, nil
}

func genDNSNamesForOpenshiftAPIServerCertKey(cfg *types.InstallConfig) ([]string, error) {
	return []string{
		fmt.Sprintf("%s-api.%s", cfg.Name, cfg.BaseDomain),
		"openshift-apiserver",
		"openshift-apiserver.kube-system",
		"openshift-apiserver.kube-system.svc",
		"openshift-apiserver.kube-system.svc.cluster.local",
		"localhost", "127.0.0.1",
	}, nil
}

func genIPAddressesForOpenshiftAPIServerCertKey(cfg *types.InstallConfig) ([]net.IP, error) {
	apiServerAddress, err := cidrhost(cfg.Networking.ServiceCIDR, 1)
	if err != nil {
		return nil, err
	}
	return []net.IP{net.ParseIP(apiServerAddress)}, nil
}

func genDNSNamesForTNCCertKey(cfg *types.InstallConfig) ([]string, error) {
	return []string{fmt.Sprintf("%s-tnc.%s", cfg.Name, cfg.BaseDomain)}, nil
}

func genSubjectForTNCCertKey(cfg *types.InstallConfig) (pkix.Name, error) {
	return pkix.Name{CommonName: fmt.Sprintf("%s-tnc.%s", cfg.Name, cfg.BaseDomain)}, nil
}

func WriteContents(st *asset.State) error {
	for _, c := range st.Contents {
		dir := filepath.Dir(c.Name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		if err := ioutil.WriteFile(c.Name, c.Data, 0644); err != nil {
			return err
		}
	}
	return nil
}
