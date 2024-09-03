package prompts

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	//"io"
	//"os"

	openai "github.com/WillChangeThisLater/go-llm/openai"
	pongo2 "github.com/flosch/pongo2/v6"
)

//go:embed schemaFiles/*.json
var schemaFS embed.FS

//go:embed promptFiles/*
var promptFS embed.FS

var prompts = map[string]Prompt{
	"json-sample-to-schema": {"json-sample-to-schema", "Turn JSON sample into formal schema OpenAI can understand and coerce results to", openai.GetModelNoError("gpt-4o-mini"), "promptFiles/describe-json", true, ""},
}

func ListPrompts() string {
	result, err := json.Marshal(prompts)
	if err != nil {
		return fmt.Sprintf("Could not get prompt info: %v", err)
	}
	return string(result)
}

type Prompt struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Model       *openai.OpenAIModel `json:"model"`
	PromptFile  string              `json:"prompt_file"`
	ForceJSON   bool                `json:"force_json"`
	SchemaFile  string              `json:"schema_file"`
}

func GetPrompt(name string) (*Prompt, error) {
	prompt, ok := prompts[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Prompt %s not found", name))
	}
	return &prompt, nil
}

// TODO: do this right
func (p *Prompt) buildPrompt(text string, imageURLs ...string) (string, error) {
	promptBytes, err := promptFS.ReadFile(p.PromptFile)
	if err != nil {
		return "", err
	}
	template, err := pongo2.FromBytes(promptBytes)
	if err != nil {
		return "", err
	}
	result, err := template.Execute(pongo2.Context{"text": text, "imageURLs": imageURLs})
	if err != nil {
		return "", err
	}

	return result, nil
}

func (p *Prompt) getSchema() (*openai.JSONSchema, error) {
	schemaBytes, err := schemaFS.ReadFile(p.SchemaFile)
	if err != nil {
		return nil, err
	}
	return &openai.JSONSchema{Name: "json_schema", Schema: schemaBytes, Strict: true}, nil
}

func (p *Prompt) Query(input string, imageURLs ...string) (string, error) {
	return Query(input, p.Name, imageURLs...)
}

func Query(input string, promptName string, imageURLs ...string) (string, error) {
	prompt, ok := prompts[promptName]
	if !ok {
		return "", errors.New(fmt.Sprintf("Could not find prompt %s", promptName))
	}

	model := prompt.Model
	var query *openai.Query
	var err error

	promptText, err := prompt.buildPrompt(input)
	if err != nil {
		return "", err
	}

	if prompt.ForceJSON {
		var schema *openai.JSONSchema
		if prompt.SchemaFile != "" {
			schema, err = prompt.getSchema()
			if err != nil {
				return "", err
			}
		}
		query, err = model.MakeJSONQuery(promptText, schema, imageURLs...)
		if err != nil {
			return "", err
		}
	} else {
		query, err = model.MakeQuery(promptText, imageURLs...)
		if err != nil {
			return "", err
		}
	}

	return query.Run()
}
