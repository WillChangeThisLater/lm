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

	openai "go-llm/openai"
	prompts "go-llm/prompts"
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
	modelPtr := flag.String("model", "gpt-4", "OpenAI model to use")
	listModelsPtr := flag.Bool("list-models", false, "List all available models")
	listPromptsPtr := flag.Bool("list-prompts", false, "List all available prompts")
	timeoutPtr := flag.Int("timeout", 60, "Timeout for reading stdin")
	jsonOutputPtr := flag.Bool("json-output", false, "If true, model will output JSON")
	jsonSchemaPtr := flag.String("json-schema-file", "", "If set, will output results based on JSON schema")
	promptPtr := flag.String("prompt", "", "If set, will feed model through a pre-defined prompt")
	promptOnlyPtr := flag.Bool("prompt-only", false, "If set, prompt will not be fed to LLM and will just be output to stdout")
	imageURLsPtr := flag.String("imageURLs", "", "Define one or more image URLs. Usage: --imageURLS \"url1,url2,url3\"")

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

	// Figure out if we want the model to use JSON or not
	outputJSON := *jsonOutputPtr
	if *jsonSchemaPtr != "" {
		outputJSON = true
	}

	// Get image URLs, if any
	imageURLs := make([]string, 0)
	for _, url := range strings.Split(*imageURLsPtr, ",") {
		url = strings.TrimSpace(url)
		if url != "" {
			imageURLs = append(imageURLs, url)
		}
	}

	// Read query from stdin
	queryString, timeoutError := readStdinWithTimeout(*timeoutPtr)
	if timeoutError != nil {
		fmt.Fprintln(os.Stderr, timeoutError)
		os.Exit(1)
	}

	// Create model
	model, err := openai.GetModel(*modelPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get model %s: %v\n", *modelPtr, err)
		os.Exit(1)
	}

	// Use prompt if we must
	if *promptPtr != "" {
		promptName := *promptPtr
		prompt, err := prompts.GetPrompt(promptName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting prompt %s: %v\n", promptName, err)
			os.Exit(1)
		}
		if prompt.Model != nil {
			if prompt.Model.ModelId != model.ModelId {
				fmt.Fprintf(os.Stderr, "Overwriting to use model %v specified by prompt %s\n", prompt.Model, promptName)
				model = prompt.Model
			}
		}
		if prompt.ForceJSON {
			outputJSON = true
		}
		queryString, err = prompt.BuildPrompt(queryString)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not format prompt %s: %v\n", promptName, err)
			os.Exit(1)

		}
	}

	// If the user only wants to see the prompt, print it and exit
	if *promptOnlyPtr {
		fmt.Println(queryString)
		os.Exit(0)
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
			}
			defer file.Close()

			schemaBytes, err := io.ReadAll(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not read json schema file %s: %v\n", jsonSchemaFile, err)
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
