package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/openshift/installer/data"
	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/terraform"
	"github.com/openshift/installer/pkg/types/config"
)

const (
	stateFileName = "terraform.state"
)

// Cluster uses the terraform executable to launch a cluster
// with the given terraform tfvar and generated templates.
type Cluster struct {
	// The root directory of the generated assets.
	rootDir    string
	tfvars     asset.Asset
	kubeconfig asset.Asset
}

var _ asset.Asset = (*Cluster)(nil)

// Name returns the human-friendly name of the asset.
func (c *Cluster) Name() string {
	return "Cluster"
}

// Dependencies returns the direct dependency for launching
// the cluster.
func (c *Cluster) Dependencies() []asset.Asset {
	return []asset.Asset{c.tfvars, c.kubeconfig}
}

// Generate launches the cluster and generates the terraform state file on disk.
func (c *Cluster) Generate(parents map[asset.Asset]*asset.State, ondisk map[string][]byte) (*asset.State, error) {
	state, ok := parents[c.tfvars]
	if !ok {
		return nil, fmt.Errorf("failed to get terraform.tfvar state in the parent asset states")
	}

	// Copy the terraform.tfvars to a temp directory where the terraform will be invoked within.
	tmpDir, err := ioutil.TempDir(os.TempDir(), "openshift-install-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := ioutil.WriteFile(filepath.Join(tmpDir, state.Contents[0].Name), state.Contents[0].Data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write terraform.tfvars file: %v", err)
	}

	var tfvars config.Cluster
	if err := json.Unmarshal(state.Contents[0].Data, &tfvars); err != nil {
		return nil, fmt.Errorf("failed to unmarshal terraform tfvars file: %v", err)
	}

	if err := data.Unpack(tmpDir); err != nil {
		return nil, err
	}

	templateDir, err := terraform.FindStepTemplates(tmpDir, terraform.InfraStep, tfvars.Platform)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("infra step not found; set OPENSHIFT_INSTALL_DATA to point to the data directory")
		}
		return nil, fmt.Errorf("error finding terraform templates: %v", err)
	}

	// take advantage of the new installer only having one step.
	err = os.Rename(filepath.Join(tmpDir, "config.tf"), filepath.Join(templateDir, "config.tf"))
	if err != nil {
		return nil, err
	}

	// This runs the terraform in a temp directory, the tfstate file will be returned
	// to the asset store to persist it on the disk.
	if err := terraform.Init(tmpDir, templateDir); err != nil {
		return nil, err
	}

	stateFile, err := terraform.Apply(tmpDir, terraform.InfraStep, templateDir)
	if err != nil {
		// we should try to fetch the terraform state file.
		log.Errorf("terraform failed: %v", err)
	}

	data, err := ioutil.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read tfstate file %q: %v", stateFile, err)
	}

	// TODO(yifan): Use the kubeconfig to verify the cluster is up.
	return &asset.State{
		Contents: []asset.Content{
			{
				Name: stateFileName,
				Data: data,
			},
		},
	}, nil
}
