package openai

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		index := seededRand.Intn(len(charset))
		sb.WriteByte(charset[index])
	}
	return sb.String()
}

func String(length int) string {
	return StringWithCharset(length, charset)
}

type Step struct {
	Explanation string `json:"explanation"`
	Output      string `json:"output"`
}

type Steps struct {
	Steps []Step `json:"steps"`
}

func TestModels(t *testing.T) {

	// we shouldn't be able to make a model with an invalid name
	_, err := GetModel("bad-model-id")
	if err == nil {
		t.Errorf("Should not have been able to create model with name 'bad-model-id'")
	}

	// we should be able to make a model with a valid name
	modelId := "gpt-3.5-turbo"
	tinyModel, err := GetModel(modelId)
	if err != nil {
		t.Errorf("Should have been able to create model 'gpt-3.5-turbo'")
	}

	// queries that are too long shouldn't be submitted
	veryLongPrompt := String(1000000)
	_, err = tinyModel.Query(veryLongPrompt)
	if err == nil {
		t.Errorf("Should not have been able to submit 1M character prompt")
	}

	prompt := "Continue the list of presidents: George Washington, John Adams, "
	result, err := tinyModel.Query(prompt)
	if err != nil {
		t.Errorf("Got error while running prompt %s against model %s: %v", prompt, modelId, err)
	}

	if !strings.Contains(strings.ToLower(result), "jefferson") {
		t.Errorf("Expected 'jefferson' to be in the result but it was not found")
	}
}

func TestJSONOutputUnstructured(t *testing.T) {
	jsonModel, _ := GetModel("gpt-4o-mini")
	badModel, _ := GetModel("gpt-3.5-turbo")

	prompt := "say hello"
	query, err := jsonModel.MakeQuery(prompt)
	if err != nil {
		t.Errorf("Could not make JSON query: %v", err)
	}
	response, err := query.Run()
	if err != nil {
		t.Errorf("Got bad response running non-JSON query against gpt-4o-mini: %v", err)
	}

	prompt = "give me a JSON sample"
	_, err = badModel.MakeJSONQuery(prompt, nil)
	if err == nil {
		t.Errorf("Should not have been able to make query with invalid model")
	}

	query, err = jsonModel.MakeJSONQuery(prompt, nil)
	if err != nil {
		t.Errorf("Could not make JSON query: %v", err)
	}
	response, err = query.Run()
	if err != nil {
		t.Errorf("Got bad response running JSON query: %v", err)
	}

	var anyJSON map[string]interface{}
	err = json.Unmarshal([]byte(response), &anyJSON)
	if err != nil {
		t.Errorf("JSON could not be unmarshaled (model returned %v)", response)
	}
}

func TestJSONOutputStructured(t *testing.T) {
	jsonModel, _ := GetModel("gpt-4o-mini")
	jsonStructure := []byte(`{"type": "object", "properties": {"steps": {"type": "array", "items": {"type": "object", "properties": {"explanation": {"type": "string"}, "output": {"type": "string"}}, "required": ["explanation", "output"], "additionalProperties": false}}, "final_answer": {"type": "string"}}, "required": ["steps", "final_answer"], "additionalProperties": false}`)
	schema := jsonSchema{Name: "test-json", Schema: jsonStructure, Strict: true}
	_, err := json.Marshal(schema)
	if err != nil {
		t.Errorf("Could not marshal JSON Schema: %v", err)
	}

	prompt := "Solve the equation: 1 + 1 * 2 / 3 - 4 / 5. Show your work in steps."
	query, err := jsonModel.MakeJSONQuery(prompt, &schema)
	if err != nil {
		t.Errorf("Could not make JSON query: %v", err)
	}
	response, err := query.Run()
	if err != nil {
		t.Errorf("Got bad response running JSON query: %v", err)
	}

	var steps Steps
	err = json.Unmarshal([]byte(response), &steps)
	if err != nil {
		t.Errorf("JSON could not be unmarshaled to steps (model returned %v)", response)
	}
}

func TestVision(t *testing.T) {
	visionModel, _ := GetModel("gpt-4o")
	imageURL := "https://upload.wikimedia.org/wikipedia/commons/thumb/e/ec/Mona_Lisa%2C_by_Leonardo_da_Vinci%2C_from_C2RMF_retouched.jpg/1024px-Mona_Lisa%2C_by_Leonardo_da_Vinci%2C_from_C2RMF_retouched.jpg"
	query, err := visionModel.MakeQuery("Who painted this?", imageURL)
	if err != nil {
		t.Errorf("Could not create vision query: %v", err)
	}
	result, err := query.Run()
	if err != nil {
		t.Errorf("Query failed: %v", err)
	}

	if !strings.Contains(strings.ToLower(result), "leonardo da vinci") {
		t.Errorf("Expected 'leonardo da vinci' to be in the result but it was not found")
	}
}
