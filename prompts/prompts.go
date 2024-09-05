package prompts

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	//"io"

	openai "github.com/WillChangeThisLater/go-llm/openai"
	pongo2 "github.com/flosch/pongo2/v6"
)

//go:embed promptFiles/*
var promptFS embed.FS

var prompts = map[string]PromptWrapper{
	"json-sample-to-schema": {"json-sample-to-schema", "Turn JSON sample into formal schema OpenAI can understand and coerce results to", "promptFiles/json-example-to-schema", true, false},
	"pdf-to-text":           {"pdf-to-text", "Convert page(s) in a PDF (represented as JPEG files based in via imageURLs...) to text", "promptFiles/pdf-to-text", true, true},
}

type PromptWrapper struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Path             string `json:"prompt_path"`
	JSONUnstructured bool   `json:"json_unstructured"`
	JSONStructured   bool   `json:"json_structured"`
}

type Prompt struct {
	Name       string              `json:"name"`
	Model      *openai.OpenAIModel `json:"model"`
	PromptFile string              `json:"prompt_file"`
	ForceJSON  bool                `json:"force_json"`
	SchemaFile string              `json:"schema_file"`
}

func (p *PromptWrapper) GetPrompt() (*Prompt, error) {
	var model *openai.OpenAIModel
	var err error
	var prompt Prompt

	// the way we get models here is kinda weird.
	// but it's good enough for now
	prompt.Name = p.Name
	if p.JSONUnstructured || p.JSONStructured {
		prompt.ForceJSON = true

		model, err = openai.GetModel("gpt-4o-mini")
		if err != nil {
			return nil, err
		}
		prompt.Model = model
	} else {
		prompt.ForceJSON = false
		model, err = openai.GetModel("gpt-4o-mini")
		if err != nil {
			return nil, err
		}
		prompt.Model = model
	}

	prompt.PromptFile = filepath.Join(p.Path, "prompt")
	if p.JSONStructured {
		prompt.SchemaFile = filepath.Join(p.Path, "schema.json")
	} else {
		prompt.SchemaFile = ""
	}

	return &prompt, nil
}

func GetPrompt(name string) (*Prompt, error) {
	promptWrapper, ok := prompts[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Prompt %s not found", name))
	}
	return promptWrapper.GetPrompt()

}

func ListPrompts() string {
	result, err := json.Marshal(prompts)
	if err != nil {
		return fmt.Sprintf("Could not get prompt info: %v", err)
	}
	return string(result)
}

// TODO: do this better
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
	schemaBytes, err := promptFS.ReadFile(p.SchemaFile)
	if err != nil {
		return nil, err
	}
	return &openai.JSONSchema{Name: "json_schema", Schema: schemaBytes, Strict: true}, nil
}

func (p *Prompt) Query(input string, imageURLs ...string) (string, error) {
	return Query(input, p.Name, imageURLs...)
}

func Query(input string, promptName string, imageURLs ...string) (string, error) {
	var model *openai.OpenAIModel
	var query *openai.Query
	var err error

	prompt, err := GetPrompt(promptName)
	if err != nil {
		return "", err
	}
	model = prompt.Model

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
