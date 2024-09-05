package prompts

import (
	"encoding/json"
	"strings"
	"testing"
)

func addTestPromptWrappers() {
	prompts["test-simple"] = PromptWrapper{"test-simple", "", "promptFiles/tests/test-simple", false, false}
	prompts["test-json-unstructured"] = PromptWrapper{"test-json-unstructured", "", "promptFiles/tests/test-json-unstructured", true, false}
	prompts["test-json-structured"] = PromptWrapper{"test-json-structured", "", "promptFiles/tests/test-json-structured", true, true}
}

func TestGetPrompt(t *testing.T) {
	//prompts["test-simple"] = PromptWrapper{"test-simple", "", "promptFiles/tests/test-simple", false, false}
	addTestPromptWrappers()

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

	//prompts["test-simple"] = PromptWrapper{"test-simple", "", "promptFiles/tests/test-simple", false, false}
	addTestPromptWrappers()

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

	//prompts["test-json-structured"] = PromptWrapper{"test-json-structured", "", "promptFiles/tests/test-json-structured", true, true}
	addTestPromptWrappers()
	prompt, err := GetPrompt("test-json-structured")
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

	//prompts["test-simple"] = PromptWrapper{"test-simple", "", "promptFiles/tests/test-simple", false, false}
	//prompts["test-json-unstructured"] = PromptWrapper{"test-json-unstructured", "", "promptFiles/tests/test-json-unstructured", true, false}
	//prompts["test-json-structured"] = PromptWrapper{"test-json-structured", "", "promptFiles/tests/test-json-structured", true, true}
	addTestPromptWrappers()

	_, err := Query("hello world", "test-simple")
	if err != nil {
		t.Errorf("Could not query model using test-simple prompt: %v", err)
	}

	result, err := Query("hello world", "test-json-unstructured")
	if err != nil {
		t.Errorf("Could not query model using unstructured JSON: %v", err)
	}
	var js json.RawMessage
	err = json.Unmarshal([]byte(result), &js)
	if err != nil {
		t.Errorf("Invalid JSON returned by unstructured JSON prompt: %v", err)
	}

	type TestResponse struct {
		Hello string `json:"hello"`
	}
	var tr TestResponse
	result, err = Query("say hello world", "test-json-structured")
	if err != nil {
		t.Errorf("Could not query model using structured JSON prompt: %v", err)
	}
	err = json.Unmarshal([]byte(result), &tr)
	if err != nil {
		t.Errorf("Invalid JSON returned by structured JSON prompt: %v", err)
	}
	if !strings.Contains(strings.ToLower(tr.Hello), "world") {
		t.Errorf("Invalid response: expected 'world' to be in response of structured JSON prompt, got %s", tr.Hello)
	}
}
