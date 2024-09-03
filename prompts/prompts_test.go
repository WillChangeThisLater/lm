package prompts

import (
	"encoding/json"
	"strings"
	"testing"

	openai "github.com/WillChangeThisLater/go-llm/openai"
)

func TestGetPrompt(t *testing.T) {
	prompts["test-simple"] = Prompt{"test-simple", "", openai.GetModelNoError("gpt-4o-mini"), "promptFiles/test-simple", false, ""}

	_, err := GetPrompt("test-simple")
	if err != nil {
		t.Errorf("Did not expect error when getting prompt: %v", err)
	}

	badPromptName := "zzz_test_does_not_exist"
	_, err = GetPrompt(badPromptName)
	if err == nil {
		t.Errorf("Should not have been able to get non existent prompt %s", badPromptName)
	}
}

func TestBuildPrompt(t *testing.T) {

	prompts["test-simple"] = Prompt{"test-simple", "", openai.GetModelNoError("gpt-4o-mini"), "promptFiles/test-simple", false, ""}

	prompt, err := GetPrompt("test-simple")
	if err != nil {
		t.Errorf("Should have been able to get test prompt: %v", err)
	}

	text := "hey there!"
	result, err := prompt.buildPrompt(text)
	if err != nil {
		t.Errorf("Did not expect error when building prompt: %v", err)
	}

	cleanResult := strings.TrimSpace(result)
	if cleanResult != text {
		t.Errorf("Template did not work (expected %s, got %s)", text, cleanResult)
	}
}

func TestGetSchema(t *testing.T) {

	prompts["test-simple-json-schema"] = Prompt{"test-simple-json-schema", "", openai.GetModelNoError("gpt-4o-mini"), "promptFiles/test-simple", true, "schemaFiles/test-simple.json"}

	prompt, err := GetPrompt("test-simple-json-schema")
	if err != nil {
		t.Errorf("Should have been able to get test prompt: %v", err)
	}

	ptr, err := prompt.getSchema()
	if err != nil {
		t.Errorf("Error getting schema for test prompt: %v", err)
	}
	if ptr == nil {
		t.Errorf("Pointer to JSON Schema object should not be nil")
	}
}

func TestQuery(t *testing.T) {

	prompts["test-simple"] = Prompt{"test-simple", "", openai.GetModelNoError("gpt-4o-mini"), "promptFiles/test-simple", false, ""}
	prompts["test-simple-json"] = Prompt{"test-simple-json", "", openai.GetModelNoError("gpt-4o-mini"), "promptFiles/test-simple", true, ""}
	prompts["test-simple-schema"] = Prompt{"test-simple-schema", "", openai.GetModelNoError("gpt-4o-mini"), "promptFiles/test-simple", true, "schemaFiles/test-simple.json"}

	_, err := Query("hello world", "test-simple")
	if err != nil {
		t.Errorf("Could not query model using test-simple prompt: %v", err)
	}

	result, err := Query("hello world", "test-simple-json")
	if err != nil {
		t.Errorf("Could not query model using test-simple-json prompt: %v", err)
	}
	var js json.RawMessage
	err = json.Unmarshal([]byte(result), &js)
	if err != nil {
		t.Errorf("Invalid JSON returned by test-simple-json prompt: %v", err)
	}

	type TestResponse struct {
		Hello string `json:"hello"`
	}
	var tr TestResponse
	result, err = Query("say hello world", "test-simple-schema")
	if err != nil {
		t.Errorf("Could not query model using test-simple-json prompt: %v", err)
	}
	err = json.Unmarshal([]byte(result), &tr)
	if err != nil {
		t.Errorf("Invalid JSON returned by test-simple-schema prompt: %v", err)
	}
	if !strings.Contains(strings.ToLower(tr.Hello), "world") {
		t.Errorf("Invalid response: expected 'world' to be in response, got %s", tr.Hello)
	}
}
