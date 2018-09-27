package asset

import (
	"io/ioutil"
	"os"

	"github.com/AlecAivazis/survey"
)

// UserProvided generates an asset that is supplied by a user.
type UserProvided struct {
	AssetName      string
	Question       *survey.Question
	EnvVarName     string
	PathEnvVarName string
}

var _ Asset = (*UserProvided)(nil)

// Dependencies returns no dependencies.
func (a *UserProvided) Dependencies() []Asset {
	return []Asset{}
}

// Generate queries for input from the user.
func (a *UserProvided) Generate(map[Asset]*State, map[string][]byte) (*State, error) {
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

	return &State{
		Contents: []Content{{
			Data: []byte(response),
		}},
	}, nil
}

// Name returns the human-friendly name of the asset.
func (a *UserProvided) Name() string {
	return a.AssetName
}
