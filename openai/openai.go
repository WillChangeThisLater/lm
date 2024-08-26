package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

type OpenAIModel struct {
	ModelId             string `json:"model_id"`
	ContextWindowSize   int    `json:"context_window_size"`
	TokenizerName       string `json:"tokenizer_name"`
	supportsJsonOutput  bool
	supportsImageOutput bool
}

var models = map[string]OpenAIModel{
	"gpt-3.5-turbo": {"gpt-3.5-turbo", 4096, "cl100k_base", false, false},
	"gpt-4":         {"gpt-4", 8192, "cl100k_base", false, false},
	"gpt-4-turbo":   {"gpt-4-turbo", 128000, "cl100k_base", false, true},
	"gpt-4o":        {"gpt-4o", 128000, "cl100k_base", false, true},
	"gpt-4o-mini":   {"gpt-4o-mini", 128000, "cl100k_base", true, true},
}

type Query struct {
	messages       []requestMessage
	responseFormat *responseFormat
	model          *OpenAIModel
}

type contentType interface{}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type imageURL struct {
	URL string `json:"url"`
}

type imageContent struct {
	Type     string   `json:"type"`
	ImageURL imageURL `json:"image_url"`
}

type requestMessage struct {
	Role    string        `json:"role"`
	Content []contentType `json:"content"`
}

type responseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type responseFormat struct {
	Type string `json:"type"`

	// optional
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

// See: https://platform.openai.com/docs/api-reference/chat/create for more info
type request struct {
	// required
	Model    string           `json:"model"`
	Messages []requestMessage `json:"messages"`

	// optional
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type choice struct {
	Index   int64
	Message responseMessage
}

type errorMessage struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type response struct {
	Id      string       `json:"id"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []choice     `json:"choices"`
	Error   errorMessage `json:"error"`
}

func getLargestModel() *OpenAIModel {
	var largestModel *OpenAIModel
	largestModelContext := 0
	for _, model := range models {
		if model.ContextWindowSize > largestModelContext {
			largestModelContext = model.ContextWindowSize
			largestModel = &model
		}
	}
	return largestModel
}

func ModelInfoString() string {
	result, err := json.Marshal(models)
	if err != nil {
		return fmt.Sprintf("Could not get model info: %v", err)
	}
	return string(result)
}

func hasModel(modelID string) bool {
	_, ok := models[modelID]
	return ok
}

// TODO: refactor this
// used by prompts.go to get model when initing a mapping
func GetModelNoError(modelID string) *OpenAIModel {
	model, ok := models[modelID]
	if !ok {
		panic("Could not find model")
	}
	return &model
}

func GetModel(modelID string) (*OpenAIModel, error) {
	model, ok := models[modelID]
	if !ok {
		modelNames := make([]string, 0)
		for key := range models {
			modelNames = append(modelNames, key)
		}
		return nil, errors.New(fmt.Sprintf("Model %s not found. valid models are %v", modelID, modelNames))
	}
	return &model, nil
}

func (m *OpenAIModel) getAPIKey() (string, error) {
	apiKey, set := os.LookupEnv("OPENAI_API_KEY")
	if !set {
		return "", errors.New("OPENAI_API_KEY not set")
	}
	return apiKey, nil
}

func (m *OpenAIModel) tokenize(query string) ([]int, error) {

	// this could change...
	tokenizer, err := tiktoken.GetEncoding(m.TokenizerName)
	if err != nil {
		return nil, err
	}
	tokens := tokenizer.Encode(query, nil, nil)
	return tokens, nil
}

func (m *OpenAIModel) countTokens(query string) (int, error) {
	tokens, err := m.tokenize(query)
	if err != nil {
		return -1, err
	}
	return len(tokens), nil
}

func (m *OpenAIModel) Query(prompt string) (string, error) {
	// Convenience method
	query, err := m.MakeQuery(prompt)
	if err != nil {
		return "", err
	}
	return query.Run()
}

func createSystemMessage(systemPrompt string) *requestMessage {
	if systemPrompt != "" {
		systemPrompt = "You are a helpful AI system"
	}
	content := contentType(textContent{Type: "text", Text: systemPrompt})
	systemMessage := requestMessage{Role: "system", Content: []contentType{content}}
	return &systemMessage
}

func createUserTextMessage(prompt string) *requestMessage {
	content := contentType(textContent{Type: "text", Text: prompt})
	systemMessage := requestMessage{Role: "user", Content: []contentType{content}}
	return &systemMessage
}

func createUserImageMessages(prompt string, imageURLs ...string) *requestMessage {
	contentList := make([]contentType, 0)
	contentList = append(contentList, contentType(textContent{Type: "text", Text: prompt}))
	for _, url := range imageURLs {
		content := imageContent{Type: "image_url", ImageURL: imageURL{URL: url}}
		contentList = append(contentList, content)

	}
	systemMessage := requestMessage{Role: "user", Content: contentList}
	return &systemMessage
}

func getJSONModelIds() []string {
	modelsWithJSON := make([]string, 0)
	for _, model := range models {
		if model.supportsJsonOutput {
			modelsWithJSON = append(modelsWithJSON, model.ModelId)
		}
	}
	return modelsWithJSON
}

func getVisionModelIds() []string {
	modelsWithVision := make([]string, 0)
	for _, model := range models {
		if model.supportsImageOutput {
			modelsWithVision = append(modelsWithVision, model.ModelId)
		}
	}
	return modelsWithVision
}

func (m *OpenAIModel) MakeQuery(prompt string, imageURLs ...string) (*Query, error) {
	// TODO: add messages for any image URLs that are passed in
	systemMessage := createSystemMessage("")
	var userMessage *requestMessage

	if len(imageURLs) > 0 && !m.supportsImageOutput {
		return nil, errors.New(fmt.Sprintf("Model %s does not support images. Models that do: %v", m.ModelId, getVisionModelIds()))
	}

	if len(imageURLs) == 0 {
		userMessage = createUserTextMessage(prompt)
	} else {
		userMessage = createUserImageMessages(prompt, imageURLs...)
	}

	q := Query{messages: []requestMessage{*systemMessage, *userMessage}, model: m}
	return &q, nil
}

func (m *OpenAIModel) MakeJSONQuery(prompt string, schema *jsonSchema, imageURLs ...string) (*Query, error) {
	// TODO: add messages for any image URLs that are passed in
	// Make sure the model supports JSON output
	if !m.supportsJsonOutput {
		return nil, errors.New(fmt.Sprintf("Model %s does not support JSON output. Models that do: %v", m.ModelId, getJSONModelIds()))
	}
	if len(imageURLs) > 0 && !m.supportsImageOutput {
		return nil, errors.New(fmt.Sprintf("Model %s does not support images. Models that do: %v", m.ModelId, getVisionModelIds()))
	}

	var jsonFormat responseFormat
	if schema == nil {
		jsonFormat = responseFormat{Type: "json_object"}
	} else {
		jsonFormat = responseFormat{Type: "json_schema", JSONSchema: schema}
	}

	systemMessage := createSystemMessage("")
	jsonMessage := createUserTextMessage("JSON output only.")
	var userMessage *requestMessage

	if len(imageURLs) == 0 {
		userMessage = createUserTextMessage(prompt)
	} else {
		userMessage = createUserImageMessages(prompt, imageURLs...)
	}
	q := Query{messages: []requestMessage{*systemMessage, *jsonMessage, *userMessage}, responseFormat: &jsonFormat, model: m}
	return &q, nil
}

func (q *Query) approxTokenCount() (int, error) {
	model := q.model
	tokenCount := 0

	for _, message := range q.messages {
		for _, content := range message.Content {
			v, ok := content.(textContent)
			if ok {
				count, err := model.countTokens(v.Text)
				if err != nil {
					return -1, err
				}
				tokenCount += count
			}

		}
	}

	approxCount := float64(tokenCount) * 1.1
	return int(approxCount), nil
}

func (q *Query) checkTokens() error {
	model := q.model
	approxTokenCount, err := q.approxTokenCount()
	if err != nil {
		return err
	}

	if approxTokenCount > model.ContextWindowSize {
		errorMessage := fmt.Sprintf("Your query has too many tokens (%d).", approxTokenCount)
		largestModel := getLargestModel()
		if largestModel.ContextWindowSize < approxTokenCount {
			errorMessage += fmt.Sprintf(" The largest model, %s, supports %d tokens", largestModel.ModelId, largestModel.ContextWindowSize)
			return errors.New(errorMessage)
		}

		biggerModels := make([]string, 0)
		for modelId, model := range models {
			contextLength := model.ContextWindowSize
			if contextLength > approxTokenCount {
				biggerModels = append(biggerModels, modelId)
			}
		}

		errorMessage += fmt.Sprintf(" Try one of these models instead: %v", biggerModels)
		return errors.New(errorMessage)
	}

	return nil
}

func (q *Query) toRequest() (*request, error) {
	model := q.model
	return &request{Model: model.ModelId, Messages: q.messages, ResponseFormat: q.responseFormat}, nil
}

func (q *Query) Run() (string, error) {
	model := q.model

	apiKey, err := model.getAPIKey()
	if err != nil {
		return "", err
	}

	err = q.checkTokens()
	if err != nil {
		return "", err
	}

	request, err := q.toRequest()
	if err != nil {
		return "", err
	}

	requestBodyAsJSON, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(requestBodyAsJSON))
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Add("Content-Type", "application/json")
	rep, err := client.Do(req)
	if err != nil {
		return "", err
	}

	responseStruct := &response{}
	contents, err := io.ReadAll(rep.Body)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(contents, responseStruct)
	if err != nil {
		return "", err
	}

	if responseStruct.Error.Message != "" {
		return "", errors.New(responseStruct.Error.Message)
	}

	return responseStruct.Choices[0].Message.Content, nil
}
