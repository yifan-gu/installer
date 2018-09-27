package ignition

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/coreos/ignition/config/util"
	ignition "github.com/coreos/ignition/config/v2_2/types"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/ignition/content"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/kubeconfig"
	"github.com/openshift/installer/pkg/asset/manifests"
	"github.com/openshift/installer/pkg/asset/tls"
	"github.com/openshift/installer/pkg/types"
)

const (
	rootDir = "/opt/tectonic"
)

// bootstrapTemplateData is the data to use to replace values in bootstrap
// template files.
type bootstrapTemplateData struct {
	ClusterDNSIP               string
	CloudProvider              string
	CloudProviderConfig        string
	DebugConfig                string
	KubeCoreRenderImage        string
	MachineConfigOperatorImage string
	EtcdCertSignerImage        string
	EtcdctlImage               string
	BootkubeImage              string
	HyperkubeImage             string
	EtcdCluster                string
}

// bootstrap is an asset that generates the ignition config for bootstrap nodes.
type bootstrap struct {
	installConfig             asset.Asset
	rootCA                    asset.Asset
	etcdCA                    asset.Asset
	ingressCertKey            asset.Asset
	kubeCA                    asset.Asset
	aggregatorCA              asset.Asset
	serviceServingCA          asset.Asset
	clusterAPIServerCertKey   asset.Asset
	etcdClientCertKey         asset.Asset
	apiServerCertKey          asset.Asset
	openshiftAPIServerCertKey asset.Asset
	apiServerProxyCertKey     asset.Asset
	adminCertKey              asset.Asset
	kubeletCertKey            asset.Asset
	mcsCertKey                asset.Asset
	serviceAccountKeyPair     asset.Asset
	kubeconfig                asset.Asset
	kubeconfigKubelet         asset.Asset
	manifests                 asset.Asset
	tectonic                  asset.Asset
	kubeCoreOperator          asset.Asset
}

var _ asset.Asset = (*bootstrap)(nil)

// newBootstrap creates a new bootstrap asset.
func newBootstrap(
	installConfigStock installconfig.Stock,
	tlsStock tls.Stock,
	kubeconfigStock kubeconfig.Stock,
	manifestStock manifests.Stock,
) *bootstrap {
	return &bootstrap{
		installConfig:             installConfigStock.InstallConfig(),
		rootCA:                    tlsStock.RootCA(),
		etcdCA:                    tlsStock.EtcdCA(),
		ingressCertKey:            tlsStock.IngressCertKey(),
		kubeCA:                    tlsStock.KubeCA(),
		aggregatorCA:              tlsStock.AggregatorCA(),
		serviceServingCA:          tlsStock.ServiceServingCA(),
		clusterAPIServerCertKey:   tlsStock.ClusterAPIServerCertKey(),
		etcdClientCertKey:         tlsStock.EtcdClientCertKey(),
		apiServerCertKey:          tlsStock.APIServerCertKey(),
		openshiftAPIServerCertKey: tlsStock.OpenshiftAPIServerCertKey(),
		apiServerProxyCertKey:     tlsStock.APIServerProxyCertKey(),
		adminCertKey:              tlsStock.AdminCertKey(),
		kubeletCertKey:            tlsStock.KubeletCertKey(),
		mcsCertKey:                tlsStock.MCSCertKey(),
		serviceAccountKeyPair:     tlsStock.ServiceAccountKeyPair(),
		kubeconfig:                kubeconfigStock.KubeconfigAdmin(),
		kubeconfigKubelet:         kubeconfigStock.KubeconfigKubelet(),
		manifests:                 manifestStock.Manifests(),
		tectonic:                  manifestStock.Tectonic(),
		kubeCoreOperator:          manifestStock.KubeCoreOperator(),
	}
}

// Dependencies returns the assets on which the bootstrap asset depends.
func (a *bootstrap) Dependencies() []asset.Asset {
	return []asset.Asset{
		a.installConfig,
		a.rootCA,
		a.etcdCA,
		a.ingressCertKey,
		a.kubeCA,
		a.aggregatorCA,
		a.serviceServingCA,
		a.clusterAPIServerCertKey,
		a.etcdClientCertKey,
		a.apiServerCertKey,
		a.openshiftAPIServerCertKey,
		a.apiServerProxyCertKey,
		a.adminCertKey,
		a.kubeletCertKey,
		a.mcsCertKey,
		a.serviceAccountKeyPair,
		a.kubeconfig,
		a.kubeconfigKubelet,
		a.manifests,
		a.tectonic,
		a.kubeCoreOperator,
	}
}

// Generate generates the ignition config for the bootstrap asset.
func (a *bootstrap) Generate(dependencies map[asset.Asset]*asset.State, ondisk map[string][]byte) (*asset.State, error) {
	installConfig, err := installconfig.GetInstallConfig(a.installConfig, dependencies)
	if err != nil {
		return nil, err
	}

	templateData, err := a.getTemplateData(installConfig)
	if err != nil {
		return nil, err
	}

	config := ignition.Config{
		Ignition: ignition.Ignition{
			Version: ignition.MaxVersion.String(),
		},
	}

	a.addBootstrapFiles(&config, dependencies)
	a.addBootkubeFiles(&config, dependencies, templateData)
	a.addTectonicFiles(&config, dependencies, templateData)
	a.addTLSCertFiles(&config, dependencies)

	config.Systemd.Units = append(
		config.Systemd.Units,
		ignition.Unit{Name: "bootkube.service", Contents: content.BootkubeSystemdContents},
		ignition.Unit{Name: "tectonic.service", Contents: content.TectonicSystemdContents, Enabled: util.BoolToPtr(true)},
		ignition.Unit{Name: "kubelet.service", Contents: applyTemplateData(content.KubeletSystemdTemplate, templateData), Enabled: util.BoolToPtr(true)},
	)

	config.Passwd.Users = append(
		config.Passwd.Users,
		ignition.PasswdUser{Name: "core", SSHAuthorizedKeys: []ignition.SSHAuthorizedKey{ignition.SSHAuthorizedKey(installConfig.Admin.SSHKey)}},
	)

	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	return &asset.State{
		Contents: []asset.Content{{
			Name: "bootstrap.ign",
			Data: data,
		}},
	}, nil
}

// Name returns the human-friendly name of the asset.
func (a *bootstrap) Name() string {
	return "Bootstrap Ignition Config"
}

// getTemplateData returns the data to use to execute bootstrap templates.
func (a *bootstrap) getTemplateData(installConfig *types.InstallConfig) (*bootstrapTemplateData, error) {
	clusterDNSIP, err := installconfig.ClusterDNSIP(installConfig)
	if err != nil {
		return nil, err
	}
	etcdEndpoints := make([]string, installConfig.MasterCount())
	for i := range etcdEndpoints {
		etcdEndpoints[i] = fmt.Sprintf("https://%s-etcd-%d.%s:2379", installConfig.Name, i, installConfig.BaseDomain)
	}
	return &bootstrapTemplateData{
		ClusterDNSIP:               clusterDNSIP,
		CloudProvider:              getCloudProvider(installConfig),
		CloudProviderConfig:        getCloudProviderConfig(installConfig),
		DebugConfig:                "",
		KubeCoreRenderImage:        "quay.io/coreos/kube-core-renderer-dev:3b6952f5a1ba89bb32dd0630faddeaf2779c9a85",
		MachineConfigOperatorImage: "docker.io/openshift/origin-machine-config-operator:v4.0.0",
		EtcdCertSignerImage:        "quay.io/coreos/kube-etcd-signer-server:678cc8e6841e2121ebfdb6e2db568fce290b67d6",
		EtcdctlImage:               "quay.io/coreos/etcd:v3.2.14",
		BootkubeImage:              "quay.io/coreos/bootkube:v0.10.0",
		HyperkubeImage:             "openshift/origin-node:latest",
		EtcdCluster:                strings.Join(etcdEndpoints, ","),
	}, nil
}

func (a *bootstrap) addBootstrapFiles(config *ignition.Config, dependencies map[asset.Asset]*asset.State) {
	config.Storage.Files = append(
		config.Storage.Files,
		fileFromBytes("/etc/kubernetes/kubeconfig", 0600, dependencies[a.kubeconfigKubelet].Contents[0].Data),
		fileFromBytes("/var/lib/kubelet/kubeconfig", 0600, dependencies[a.kubeconfigKubelet].Contents[0].Data),
	)
	config.Storage.Files = append(
		config.Storage.Files,
		filesFromContents(rootDir, 0644, dependencies[a.kubeCoreOperator].Contents)...,
	)
}

func (a *bootstrap) addBootkubeFiles(config *ignition.Config, dependencies map[asset.Asset]*asset.State, templateData *bootstrapTemplateData) {
	config.Storage.Files = append(
		config.Storage.Files,
		fileFromString("/opt/tectonic/bootkube.sh", 0555, applyTemplateData(content.BootkubeShFileTemplate, templateData)),
	)
	config.Storage.Files = append(
		config.Storage.Files,
		filesFromContents(rootDir, 0600, dependencies[a.kubeconfig].Contents)...,
	)
	config.Storage.Files = append(
		config.Storage.Files,
		filesFromContents(rootDir, 0644, dependencies[a.manifests].Contents)...,
	)
}

func (a *bootstrap) addTectonicFiles(config *ignition.Config, dependencies map[asset.Asset]*asset.State, templateData *bootstrapTemplateData) {
	config.Storage.Files = append(
		config.Storage.Files,
		fileFromString("/opt/tectonic/tectonic.sh", 0555, content.TectonicShFileContents),
	)
	config.Storage.Files = append(
		config.Storage.Files,
		filesFromContents(rootDir, 0644, dependencies[a.tectonic].Contents)...,
	)
}

func (a *bootstrap) addTLSCertFiles(config *ignition.Config, dependencies map[asset.Asset]*asset.State) {
	for _, asset := range []asset.Asset{
		a.rootCA,
		a.kubeCA,
		a.aggregatorCA,
		a.serviceServingCA,
		a.etcdCA,
		a.clusterAPIServerCertKey,
		a.etcdClientCertKey,
		a.apiServerCertKey,
		a.openshiftAPIServerCertKey,
		a.apiServerProxyCertKey,
		a.adminCertKey,
		a.kubeletCertKey,
		a.mcsCertKey,
		a.serviceAccountKeyPair,
	} {
		config.Storage.Files = append(config.Storage.Files, filesFromContents(rootDir, 0600, dependencies[asset].Contents)...)
	}

	config.Storage.Files = append(
		config.Storage.Files,
		fileFromBytes("/etc/ssl/etcd/ca.crt", 0600, dependencies[a.etcdClientCertKey].Contents[tls.CertIndex].Data),
	)
}

func getCloudProvider(installConfig *types.InstallConfig) string {
	if installConfig.AWS != nil {
		return "aws"
	}
	return ""
}

func getCloudProviderConfig(installConfig *types.InstallConfig) string {
	return ""
}

func applyTemplateData(template *template.Template, templateData interface{}) string {
	buf := &bytes.Buffer{}
	if err := template.Execute(buf, templateData); err != nil {
		panic(err)
	}
	return buf.String()
}
