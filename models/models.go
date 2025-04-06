package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type Model struct {
	Provider                 string `json:"provider"`
	ModelId                  string `json:"model_id"`
	ContextWindowSize        int    `json:"context_window_size"`
	TokenizerName            string `json:"tokenizer_name"`
	SupportsImageOutput      bool   `json:"supports_image"`
	SupportsUnstructuredJson bool   `json:"supports_unstructured_json"`
	SupportsStructuredJson   bool   `json:"supports_structured_json"`
}

var models = map[string]Model{
	"gpt-3.5-turbo":     {"openai", "gpt-3.5-turbo", 4096, "cl100k_base", false, false, false},
	"gpt-4":             {"openai", "gpt-4", 8192, "cl100k_base", false, false, false},
	"gpt-4o":            {"openai", "gpt-4o", 128000, "cl100k_base", true, true, false},
	"gpt-4-turbo":       {"openai", "gpt-4-turbo", 128000, "cl100k_base", true, true, false},
	"gpt-4o-mini":       {"openai", "gpt-4o-mini", 128000, "cl100k_base", true, true, true},
	"local-deepseek-7b": {"local", "deepseek-7b", 8192, "cl100k_base", false, false, false},
	"aws-nova-lite":     {"aws", "us.amazon.nova-lite-v1:0", 300000, "cl100k_base", true, false, false},
	"aws-nova-pro":     {"aws", "us.amazon.nova-pro-v1:0", 300000, "cl100k_base", true, false, false},
}

type Query struct {
	messages       []requestMessage
	responseFormat *responseFormat
	model          *Model
}

type contentType interface{}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type ImageContent struct {
	Type     string   `json:"type"`
	ImageURL ImageURL `json:"image_url"`
	ImageContents []byte `json:"-"`
}

type requestMessage struct {
	Role    string        `json:"role"`
	Content []contentType `json:"content"`
}

type responseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type JSONSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type responseFormat struct {
	Type string `json:"type"`

	// optional
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
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

func getLargestModel() *Model {
	var largestModel *Model
	largestModelContext := 0
	for _, model := range models {
		if model.ContextWindowSize > largestModelContext {
			largestModelContext = model.ContextWindowSize
			largestModel = &model
		}
	}
	return largestModel
}

func (m *Model) FlightCheck(needsImage bool, needsUnstructuredJSON bool, needsStructuredJSON bool) (bool, string) {
	if needsImage && !m.SupportsImageOutput {
		return false, "Model does not support image output"
	}
	if needsUnstructuredJSON && !m.SupportsUnstructuredJson {
		return false, "Model does not support unstructured JSON"
	}
	if needsStructuredJSON && !m.SupportsStructuredJson {
		return false, "Model does not support structured JSON"
	}
	return true, ""
}

// TODO: maybe add context here?
func SuggestedModel(needsImage bool, needsUnstructuredJSON bool, needsStructuredJSON bool) (*Model, error) {
	for _, model := range models {
		valid, _ := model.FlightCheck(needsImage, needsUnstructuredJSON, needsStructuredJSON)
		if valid {
			return &model, nil
		}
	}
	return nil, errors.New("Could not find suggested model given your constraints")
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
func GetModelNoError(modelID string) *Model {
	model, ok := models[modelID]
	if !ok {
		panic("Could not find model")
	}
	return &model
}

func GetModel(modelID string) (*Model, error) {
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

func (m *Model) getAPIKey() (string, error) {
	provider := m.Provider
	if provider == "openai" {
		apiKey, set := os.LookupEnv("OPENAI_API_KEY")
		if !set {
			return "", errors.New("OPENAI_API_KEY not set")
		}
		return apiKey, nil
	} else if provider == "local" {
		return "", nil
	} else if provider == "aws" {
		return "", nil
	} else {
		return "", errors.New(fmt.Sprintf("Provider not found: %s", provider))
	}
}

func newAWSClient() (*bedrockruntime.Client, error) {
	// Load the Shared AWS Configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"), // Specify the region
	)
	if err != nil {
		return nil, err
	}

	return bedrockruntime.NewFromConfig(cfg), nil
}

// AI gen
func (m *Model) RunAWSClient(query *Query) (string, error) {

	client, err := newAWSClient()
	if err != nil {
		return "", fmt.Errorf("failed to create AWS client: %w", err)
	}

	// ignore the initial system message
	messages := make([]types.Message, len(query.messages)-1)
	for i, msg := range query.messages[1:] {
		var content []types.ContentBlock
		for _, item := range msg.Content {
			switch v := item.(type) {
			case textContent:
				if v.Text == "" {
					return "", errors.New("Message text cannot be empty")
				}
				content = append(content, &types.ContentBlockMemberText{
					Value: v.Text,
				})

			// TODO: add support for image content
			// apparently ImageSource needs to be an array of bytes
			// (right now we are using an image URL for everything)
			case ImageContent:
				//imagePath := "/Users/paul.wendt/Downloads/rome.jpg"
	            //fileData, err := os.ReadFile(imagePath)
	            //if err != nil {
	            //	return "", errors.New(fmt.Sprintf("Could not read file: %v\n", err))
	            //}
				//

				// TODO: figure this out just from the image contents
				fileData := v.ImageContents
				mimeType := http.DetectContentType(fileData)

	            //ext := filepath.Ext(imagePath)
	            //mimeType := mime.TypeByExtension(ext)
				mimeType = strings.Replace(mimeType, "image/", "", 1)
	            if mimeType == "" {
	            	return "", errors.New(fmt.Sprintf("Unsupported file format %s\n", mimeType))
	            }

                content = append(content, &types.ContentBlockMemberImage{
                        Value: types.ImageBlock{
						Source: &types.ImageSourceMemberBytes{Value: fileData},
                                Format: types.ImageFormat(mimeType), // TODO determine this from the image itself
                        },
                })
			}

		}
		// Convert string to ConversationRole
		role := strings.ToLower(msg.Role)
		if role == "system" {
			role = "assistant"
		}

		convertedRole := types.ConversationRole(role)
		messages[i] = types.Message{
			Role:    convertedRole,
			Content: content,
		}
	}

	// Construct the request
	input := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(m.ModelId),
		Messages: messages,
	}

	// Invoke the API
	result, err := client.Converse(context.TODO(), input)
	if err != nil {
		return "", fmt.Errorf("failed to invoke Converse API: %w", err)
	}

	var messageContent strings.Builder // Use a builder for better efficiency

	switch v := (result.Output).(type) {
	case *types.ConverseOutputMemberMessage:
		for _, block := range v.Value.Content {
			switch b := block.(type) {
			case *types.ContentBlockMemberText:
				messageContent.WriteString(b.Value)
			}
		}
	default:
		return "", fmt.Errorf("unexpected result type")
	}

	return messageContent.String(), nil

}

func (m *Model) getEndpoint() (string, error) {
	if m.Provider == "openai" {
		return "https://api.openai.com/v1/chat/completions", nil
	} else if m.Provider == "local" {
		return "http://localhost:8080/v1/chat/completions", nil
	} else if m.Provider == "aws" {
		return "", errors.New(fmt.Sprintf("Calls to AWS provider should use the Go SDK"))
	} else {
		return "", errors.New(fmt.Sprintf("Provider not found: %s", m.Provider))
	}
}

func (m *Model) tokenize(query string) ([]int, error) {

	// this could change...
	tokenizer, err := tiktoken.GetEncoding(m.TokenizerName)
	if err != nil {
		return nil, err
	}
	tokens := tokenizer.Encode(query, nil, nil)
	return tokens, nil
}

func (m *Model) countTokens(query string) (int, error) {
	tokens, err := m.tokenize(query)
	if err != nil {
		return -1, err
	}
	return len(tokens), nil
}

func (m *Model) Query(prompt string) (string, error) {
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
	userMessage := requestMessage{Role: "user", Content: []contentType{content}}
	return &userMessage
}

func createUserImageMessages(prompt string, imageContents ...ImageContent) *requestMessage {
	contentList := make([]contentType, 0)
	contentList = append(contentList, contentType(textContent{Type: "text", Text: prompt}))
	for _, content := range imageContents {
		contentList = append(contentList, content)

	}
	systemMessage := requestMessage{Role: "user", Content: contentList}
	return &systemMessage
}

func getJSONModelIds(structured bool) []string {
	modelsWithJSON := make([]string, 0)
	for _, model := range models {
		if structured && model.SupportsStructuredJson {
			modelsWithJSON = append(modelsWithJSON, model.ModelId)
		} else if !structured && model.SupportsUnstructuredJson {
			modelsWithJSON = append(modelsWithJSON, model.ModelId)
		}
	}
	return modelsWithJSON
}

func getVisionModelIds() []string {
	modelsWithVision := make([]string, 0)
	for _, model := range models {
		if model.SupportsImageOutput {
			modelsWithVision = append(modelsWithVision, model.ModelId)
		}
	}
	return modelsWithVision
}

func (m *Model) MakeQuery(prompt string, imageContent ...ImageContent) (*Query, error) {
	// TODO: add messages for any image URLs that are passed in
	systemMessage := createSystemMessage("You are a friendly assistant")
	var userMessage *requestMessage

	if len(imageContent) > 0 && !m.SupportsImageOutput {
		return nil, errors.New(fmt.Sprintf("Model %s does not support images. Models that do: %v", m.ModelId, getVisionModelIds()))
	}

	if len(imageContent) == 0 {
		userMessage = createUserTextMessage(prompt)
	} else {
		userMessage = createUserImageMessages(prompt, imageContent...)
	}

	q := Query{messages: []requestMessage{*systemMessage, *userMessage}, model: m}
	return &q, nil
}

func (m *Model) MakeJSONQuery(prompt string, schema *JSONSchema, imageContent ...ImageContent) (*Query, error) {
	if len(imageContent) > 0 && !m.SupportsImageOutput {
		return nil, errors.New(fmt.Sprintf("Model %s does not support images. Models that do: %v", m.ModelId, getVisionModelIds()))
	}

	var jsonFormat responseFormat
	if schema == nil {
		if !m.SupportsUnstructuredJson {
			return nil, errors.New(fmt.Sprintf("Model %s does not support unstructured JSON output. Models that might: %v", m.ModelId, getJSONModelIds(false)))
		}
		jsonFormat = responseFormat{Type: "json_object"}
	} else {
		if !m.SupportsStructuredJson {
			return nil, errors.New(fmt.Sprintf("Model %s does not support structured JSON output. Models that might: %v", m.ModelId, getJSONModelIds(true)))
		}
		jsonFormat = responseFormat{Type: "json_schema", JSONSchema: schema}
	}

	systemMessage := createSystemMessage("")
	jsonMessage := createUserTextMessage("JSON output only.")
	var userMessage *requestMessage

	if len(imageContent) == 0 {
		userMessage = createUserTextMessage(prompt)
	} else {
		userMessage = createUserImageMessages(prompt, imageContent...)
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

	if model.Provider == "aws" {
		return model.RunAWSClient(q)
	}

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
    //jsonString := string(requestBodyAsJSON)
    //fmt.Println(jsonString)

	client := &http.Client{}
	endpoint, err := model.getEndpoint()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(requestBodyAsJSON))
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
