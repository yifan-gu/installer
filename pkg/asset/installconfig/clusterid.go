package installconfig

import (
	"github.com/pborman/uuid"

	"github.com/openshift/installer/pkg/asset"
)

type clusterID struct{}

var _ asset.Asset = (*clusterID)(nil)

// Dependencies returns no dependencies.
func (a *clusterID) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate generates a new UUID
func (a *clusterID) Generate(dependencies map[asset.Asset]*asset.State, ondisk map[string][]byte) (*asset.State, error) {
	// Short-circuit if install-config already exisits.
	if _, ok := ondisk[installCfgFilename]; ok {
		return nil, nil
	}

	return &asset.State{
		Contents: []asset.Content{
			{Data: []byte(uuid.New())},
		},
	}, nil
}

// Name returns the human-friendly name of the asset.
func (a *clusterID) Name() string {
	return "Cluster ID"
}
