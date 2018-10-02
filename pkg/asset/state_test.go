package asset

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatePersistToFile(t *testing.T) {
	cases := []struct {
		name      string
		filenames []string
	}{
		{
			name:      "no files",
			filenames: []string{""},
		},
		{
			name:      "single file",
			filenames: []string{"file1"},
		},
		{
			name:      "multiple files",
			filenames: []string{"file1", "file2"},
		},
		{
			name:      "new directory",
			filenames: []string{"dir1/file1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "TestStatePersistToFile")
			if err != nil {
				t.Skipf("could not create temporary directory: %v", err)
			}
			defer os.RemoveAll(dir)

			state := &State{
				Contents: make([]Content, len(tc.filenames)),
			}
			expectedFiles := map[string][]byte{}
			for i, filename := range tc.filenames {
				data := []byte(fmt.Sprintf("data%d", i))
				state.Contents[i].Name = filename
				state.Contents[i].Data = data
				if filename != "" {
					expectedFiles[filepath.Join(dir, filename)] = data
				}
			}
			err = state.PersistToFile(dir)
			assert.NoError(t, err, "unexpected error persisting state to file")
			verifyFilesCreated(t, dir, expectedFiles)
		})
	}
}

func verifyFilesCreated(t *testing.T, dir string, expectedFiles map[string][]byte) {
	dirContents, err := ioutil.ReadDir(dir)
	assert.NoError(t, err, "could not read contents of directory %q", dir)
	for _, fileinfo := range dirContents {
		fullPath := filepath.Join(dir, fileinfo.Name())
		if fileinfo.IsDir() {
			verifyFilesCreated(t, fullPath, expectedFiles)
		} else {
			expectedData, fileExpected := expectedFiles[fullPath]
			if !fileExpected {
				t.Errorf("Unexpected file created: %v", fullPath)
				continue
			}
			actualData, err := ioutil.ReadFile(fullPath)
			assert.NoError(t, err, "unexpected error reading created file %q", fullPath)
			assert.Equal(t, expectedData, actualData, "unexpected data in created file %q", fullPath)
			delete(expectedFiles, fullPath)
		}
	}
	for f := range expectedFiles {
		t.Errorf("Expected file %q not created", f)
	}
}

func TestLoadFileWithPattern(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string][]byte
		pattern   string
		state     *State
		expectOK  bool
		expectErr bool
	}{
		{
			name:      "only dirs",
			files:     nil,
			pattern:   "",
			state:     (*State)(nil),
			expectOK:  false,
			expectErr: false,
		},
		{
			name:      "pattern empty",
			files:     map[string][]byte{"foo.bar": []byte("some data")},
			pattern:   "",
			state:     (*State)(nil),
			expectOK:  false,
			expectErr: false,
		},
		{
			name:      "pattern doesn't match",
			files:     map[string][]byte{"foo.bar": []byte("some data")},
			pattern:   "bar.foo",
			state:     (*State)(nil),
			expectOK:  false,
			expectErr: false,
		},
		{
			name:    "with contents",
			files:   map[string][]byte{"foo.bar": []byte("some data")},
			pattern: "foo.bar",
			state: &State{
				Contents: []Content{
					{
						Name: "foo.bar",
						Data: []byte("some data"),
					},
				},
			},
			expectOK:  true,
			expectErr: false,
		},
		{
			name:    "match one file",
			files:   map[string][]byte{"foo.bar": []byte("some data")},
			pattern: "foo.bar",
			state: &State{
				Contents: []Content{
					{
						Name: "foo.bar",
						Data: []byte("some data"),
					},
				},
			},
			expectOK:  true,
			expectErr: false,
		},
		{
			name: "match multiple files",
			files: map[string][]byte{
				"foo-10.bar": []byte("some data 10"),
				"foo-0.bar":  []byte("some data 0"),
				"foo-1.bar":  []byte("some data 1"),
				"foo-.bar":   []byte("some data 2"),
				"foo-b.bar":  []byte("some data 3"),
				"foo-1.ba":   []byte("some data 4"),
			},
			pattern: "foo-[0-9]*.bar",
			state: &State{
				Contents: []Content{
					{
						Name: "foo-0.bar",
						Data: []byte("some data 0"),
					},
					{
						Name: "foo-1.bar",
						Data: []byte("some data 1"),
					},
					{
						Name: "foo-10.bar",
						Data: []byte("some data 10"),
					},
				},
			},
			expectOK:  true,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, ok, err := LoadFileWithPattern(tt.files)(tt.pattern)
			assert.Equal(t, tt.expectErr, (err != nil))
			assert.Equal(t, tt.expectOK, ok)
			assert.Equal(t, tt.state, state)
		})
	}
}
