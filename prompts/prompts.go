package prompts

import (
	"errors"
	"fmt"
	"io"
	"os"

	openai "go-openai-cli/openai"
)

var prompts = map[string]Prompt{
	"describe-json": {"Turn JSON sample into formal schema OpenAI can understand and coerce results to", openai.GetModelNoError("gpt-4o-mini"), true, describeJSON},
}

type Prompt struct {
	Description string              `json:"description"`
	Model       *openai.OpenAIModel `json:"model"`
	ForceJSON   bool                `json:"force_json"`
	BuildPrompt func(string) (string, error)
}

func GetPrompt(name string) (*Prompt, error) {
	prompt, ok := prompts[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Prompt %s not found", name))
	}
	return &prompt, nil
}

func describeJSON(input string) (string, error) {
	file, err := os.Open("prompts/describe-json.txt")
	if err != nil {
		return "", err
	}

	defer file.Close()

	template, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf("%s\n\n```json\n%s\n```", string(template), input)
	return prompt, nil
}
