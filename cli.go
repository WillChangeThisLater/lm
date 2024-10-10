package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	openai "github.com/WillChangeThisLater/lm/openai"
	prompts "github.com/WillChangeThisLater/lm/prompts"
	utils "github.com/WillChangeThisLater/lm/utils"
)

func readStdin() string {
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, os.Stdin)
	return buf.String()
}

func readStdinWithTimeout(timeout int) (string, error) {
	ch := make(chan string)
	go func() {
		ch <- readStdin()
	}()
	select {
	case query := <-ch:
		return query, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		return "", errors.New(fmt.Sprintf("Hit timeout (%d secs) reading stdin", timeout))
	}
}

func main() {
	// Define flags
	modelPtr := flag.String("model", "gpt-4o", "OpenAI model to use")
	listModelsPtr := flag.Bool("list-models", false, "List all available models")
	listPromptsPtr := flag.Bool("list-prompts", false, "List all available prompts")
	timeoutPtr := flag.Int("timeout", 60, "Timeout for reading stdin")
	jsonOutputPtr := flag.Bool("json-output", false, "If true, model will output JSON")
	jsonSchemaPtr := flag.String("json-schema-file", "", "If set, will output results based on JSON schema")
	promptPtr := flag.String("prompt", "", "If set, will feed model through a pre-defined prompt")
	promptOnlyPtr := flag.Bool("prompt-only", false, "If set, prompt will not be fed to LLM and will just be output to stdout")
	imageURLsPtr := flag.String("imageURLs", "", "Define one or more image URLs. Usage: --imageURLS \"url1,url2,url3\"")
	imageFilesPtr := flag.String("imageFiles", "", "Define one or more image files. Usage: --imageFiles \"file1,file2,file3\"")
	screenshotPtr := flag.Bool("screenshot", false, "If set, screenshots of all monitors will be taken and used as image file input")

	// Parse flags
	flag.Parse()

	// If --list-models is set, just list the models and exit
	if *listModelsPtr {
		fmt.Println(openai.ModelInfoString())
		os.Exit(0)
	}

	// If --list-prompts is set, just list the prompts and exit
	if *listPromptsPtr {
		fmt.Println(prompts.ListPrompts())
		os.Exit(0)
	}

	// Create model
	model, err := openai.GetModel(*modelPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get model %s: %v\n", *modelPtr, err)
		os.Exit(1)
	}

	// Figure out if we want JSON output or not
	// if --json-output isn't set but --json-schema-file is, we assume the
	// user (me!) wants JSON output
	outputJSON := *jsonOutputPtr
	if *jsonSchemaPtr != "" {
		outputJSON = true
	}

	// Collect image URLs, if any
	imageURLs := make([]string, 0)
	for _, url := range strings.Split(*imageURLsPtr, ",") {
		url = strings.TrimSpace(url)
		if url != "" {
			imageURLs = append(imageURLs, url)
		}
	}

	// To the existing url list, add encoded images
	for _, fileName := range strings.Split(*imageFilesPtr, ",") {
		fileName = strings.TrimSpace(fileName)
		if fileName != "" {
			url, err := utils.GetImageURL(fileName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not generate base64 encoding for file %s: %v\n", fileName, err)
				os.Exit(1)
			}
			imageURLs = append(imageURLs, url)
		}
	}

	if *screenshotPtr {
		screenshots, err := utils.TakeScreenshots()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not generate screenshots: %v\n", err)
			os.Exit(1)
		}
		for _, fileName := range screenshots {
			url, err := utils.GetImageURL(fileName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not generate base64 encoding for screenshot %s: %v\n", fileName, err)
				os.Exit(1)
			}
			imageURLs = append(imageURLs, url)
		}
	}

	// TODO: for some specific prompts we may not want to do this...
	// Read query from stdin
	queryString, timeoutError := readStdinWithTimeout(*timeoutPtr)
	if timeoutError != nil {
		fmt.Fprintln(os.Stderr, timeoutError)
		os.Exit(1)
	}

	// If the user only wants to see the prompt, print it and exit
	if *promptOnlyPtr {
		fmt.Println(queryString)
		os.Exit(0)
	}

	// Use prompt if we must
	if *promptPtr != "" {
		response, err := prompts.Query(queryString, *promptPtr, imageURLs...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query using prompt %s failed: %v\n", *promptPtr, err)
			os.Exit(1)
		}
		fmt.Println(response)
		os.Exit(0)
	}

	// flight check!
	needsImageOutput := len(imageURLs) > 0
	needsUnstructuredJson := *jsonOutputPtr
	needsStructuredJson := *jsonSchemaPtr != ""
	validModel, reason := model.FlightCheck(needsImageOutput, needsUnstructuredJson, needsStructuredJson)
	if !validModel {
		fmt.Fprintf(os.Stderr, "Model %s cannot be used for your query: %s\n", model.ModelId, reason)
		suggestedModel, err := openai.SuggestedModel(needsImageOutput, needsUnstructuredJson, needsStructuredJson)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not find model to use")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, fmt.Sprintf("Using model %s\n", suggestedModel.ModelId))
		model = suggestedModel
	}

	// create the query object
	var query *openai.Query
	if outputJSON {
		jsonSchemaFile := *jsonSchemaPtr
		var schema *openai.JSONSchema
		if jsonSchemaFile != "" {
			file, err := os.Open(jsonSchemaFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not open json schema file %s: %v\n", jsonSchemaFile, err)
				os.Exit(1)
			}
			defer file.Close()

			schemaBytes, err := io.ReadAll(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not read json schema file %s: %v\n", jsonSchemaFile, err)
				os.Exit(1)
			}
			schema = &openai.JSONSchema{Name: "json_schema", Schema: schemaBytes, Strict: true}
		}
		query, err = model.MakeJSONQuery(queryString, schema, imageURLs...)

	} else {
		query, err = model.MakeQuery(queryString, imageURLs...)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create query: %v\n", err)
		os.Exit(1)
	}

	response, err := query.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying model: %v\n", err)
		os.Exit(1)
	}

	// Write the response
	fmt.Println(response)
}
