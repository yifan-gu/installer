package asset

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

// State is the state of an Asset.
type State struct {
	Contents []Content
}

// Content is a generated portion of an Asset.
type Content struct {
	Name string // the path on disk for this content.
	Data []byte
}

// ReadAllFiles returns a map of filename path (relative to the clusterDir)
// to the state
func ReadAllFiles(clusterDir string) (map[string][]byte, error) {
	fileMap := make(map[string][]byte)

	// Don't bother if the clusterDir is not created yet because that
	// means there's no assets generated yet.
	_, err := os.Stat(clusterDir)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}

	if err := filepath.Walk(clusterDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		filename, err := filepath.Rel(clusterDir, path)
		if err != nil {
			return err
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		fileMap[filename] = data

		return nil

	}); err != nil {
		return nil, err
	}
	return fileMap, nil
}

// ByContentName will implement the Sort interface on State,
// it will sort it by the Content's name in lexicographic order.
type ByContentName State

// Len returns the lens of the contents of the state.
func (b ByContentName) Len() int {
	return len(b.Contents)
}

// Swap swaps the order of state.Contents[i] and state.Contests[j].
func (b ByContentName) Swap(i, j int) {
	b.Contents[i], b.Contents[j] = b.Contents[j], b.Contents[i]
}

// Less returns true if state.Contents[i]'s name is less than state.Contents[i]'s name lexicographically.
func (b ByContentName) Less(i, j int) bool {
	return b.Contents[i].Name < b.Contents[j].Name
}

// PersistToFile persists the data in the State to files. Each Content entry that
// has a non-empty Name will be persisted to a file with that name.
func (s *State) PersistToFile(directory string) error {
	for _, c := range s.Contents {
		if c.Name == "" {
			continue
		}
		path := filepath.Join(directory, c.Name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(path, c.Data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func LoadFileWithPattern(fileMap map[string][]byte) PatternFetcher {
	return func(pattern string) (*State, bool, error) {
		var state State

		for filename, data := range fileMap {
			match, err := filepath.Match(pattern, filename)
			if err != nil {
				return nil, false, err
			}

			if match {
				state.Contents = append(state.Contents, Content{
					Name: filename,
					Data: data,
				})
			}
		}

		sort.Sort(ByContentName(state))

		if len(state.Contents) > 0 {
			return &state, true, nil
		}

		return nil, false, nil
	}
}
