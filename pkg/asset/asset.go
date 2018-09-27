package asset

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Asset used to install OpenShift.
type Asset interface {
	// Dependencies returns the assets upon which this asset directly depends.
	Dependencies() []Asset

	// Generate generates this asset given the states of its dependent assets
	// and the on-disk assets.
	// If the the asset already exists on disk, then it will use it directly.
	Generate(dependency map[Asset]*State, onDisk map[string][]byte) (*State, error)

	// Name returns the human-friendly name of the asset.
	Name() string
}

// GetDataByFilename searches the file in the asset.State.Contents, and returns its data.
// filename is the base name of the file.
func GetDataByFilename(a Asset, parents map[Asset]*State, filename string) ([]byte, error) {
	st, ok := parents[a]
	if !ok {
		return nil, fmt.Errorf("failed to find %T in parents", a)
	}

	for _, c := range st.Contents {
		if filepath.Base(c.Name) == filename {
			return c.Data, nil
		}
	}
	return nil, fmt.Errorf("failed to find data in %v with filename == %q", st, filename)
}

// LoadOnDiskFiles returns a map that contains all the files in the directory.
// The key is the relative path of the file to the directory.
func LoadOnDiskFiles(dir string) (map[string][]byte, error) {
	assets := make(map[string][]byte)

	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Run stat since the info can be a symlink.
		info, err = os.Stat(path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("failed to calculate the relpath: %v", err)
		}

		// TODO(yifan): Also skip the hidden state file here.
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read asset %q: %v", path, err)
		}

		assets[relPath] = data

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk through the directory to load exisiting assets: %v", err)
	}

	return assets, nil
}
