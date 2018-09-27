package installconfig

import (
	"io/ioutil"
	"os"

	"github.com/AlecAivazis/survey"
	"github.com/openshift/installer/pkg/asset"
)

// UserProvided generates an asset that is supplied by a user.
type UserProvided struct {
	AssetName      string
	Question       *survey.Question
	EnvVarName     string
	PathEnvVarName string
}

var _ asset.Asset = (*UserProvided)(nil)

// Dependencies returns no dependencies.
func (a *UserProvided) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate queries for input from the user.
func (a *UserProvided) Generate(dependencies map[asset.Asset]*asset.State, ondisk map[string][]byte) (*asset.State, error) {
	// Short-circuit if install-config already exisits.
	if _, ok := ondisk[installCfgFilename]; ok {
		return nil, nil
	}

	var response string

	if value, ok := os.LookupEnv(a.EnvVarName); ok {
		response = value
	} else if path, ok := os.LookupEnv(a.PathEnvVarName); ok {
		value, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		response = string(value)
	}

	if response == "" {
		survey.AskOne(a.Question.Prompt, &response, a.Question.Validate)
	} else if a.Question.Validate != nil {
		if err := a.Question.Validate(response); err != nil {
			return nil, err
		}
	}

	return &asset.State{
		Contents: []asset.Content{{
			Data: []byte(response),
		}},
	}, nil
}

// Name returns the human-friendly name of the asset.
func (a *UserProvided) Name() string {
	return a.AssetName
}
