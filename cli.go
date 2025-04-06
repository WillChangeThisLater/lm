package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	models "github.com/WillChangeThisLater/lm/models"
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
	modelPtr := flag.String("model", "gpt-4o", "model to use")
	listModelsPtr := flag.Bool("list-models", false, "List all available models")
	timeoutPtr := flag.Int("timeout", 60, "Timeout for reading stdin")
	promptPtr := flag.String("prompt", "", "Append prompt to stdin")
	imageURLsPtr := flag.String("imageURLs", "", "Define one or more image URLs. Usage: --imageURLS \"url1,url2,url3\"")
	imageFilesPtr := flag.String("imageFiles", "", "Define one or more image files. Usage: --imageFiles \"file1,file2,file3\"")
	screenshotPtr := flag.Bool("screenshot", false, "If set, screenshots of all monitors will be taken and used as image file input")
	sitesPtr := flag.String("sites", "", "Define one or more sites to scrape")
	cachePtr := flag.Bool("cache", false, "Enable persistent cache")

	// Parse flags
	flag.Parse()

	// If --list-models is set, just list the models and exit
	if *listModelsPtr {
		fmt.Println(models.ModelInfoString())
		os.Exit(0)
	}

	// Create model
	model, err := models.GetModel(*modelPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get model %s: %v\n", *modelPtr, err)
		os.Exit(1)
	}

	// figure out the location of the cache
	usr, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error fetching user details:", err)
		os.Exit(1)
	}
	defaultCachePath := filepath.Join(usr.HomeDir, ".cache", "your_program_name")

	// get the cache
	var cache *utils.Cache
	if *cachePtr {
		var err error
		cache, err = utils.NewCache(defaultCachePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error initializing cache:", err)
			os.Exit(1)
		}
		defer cache.Close()
	}

	// Collect image URLs, if any
	images := make([]models.ImageContent, 0)
	for _, url := range strings.Split(*imageURLsPtr, ",") {
		url = strings.TrimSpace(url)
		if url != "" {
			imageContent := models.ImageContent{Type: "image_url", ImageURL: models.ImageURL{URL: url}}
			images = append(images, imageContent)
		}
	}

	// To the existing url list, add encoded images
	for _, fileName := range strings.Split(*imageFilesPtr, ",") {
		fileName = strings.TrimSpace(fileName)
		if fileName != "" {

			imageContentPtr, err := utils.GetImageContent(fileName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not generate base64 encoding for file %s: %v\n", fileName, err)
				os.Exit(1)
			}
			images = append(images, *imageContentPtr)
		}
	}

	// Take screenshots of sites, if needed
	urls := make([]string, 0)
	for _, url := range strings.Split(*sitesPtr, ",") {
		url = strings.TrimSpace(url)
		if url != "" {
			urls = append(urls, url)
		}
	}
	if len(urls) > 0 {
		fileNames, err := utils.SiteScreenshots(urls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not generate screenshots for %+v: %v\n", fileNames, err)
			os.Exit(1)
		}
		for _, fileName := range fileNames {
			imageContentPtr, err := utils.GetImageContent(fileName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not generate base64 encoding for file %s: %v\n", fileName, err)
				os.Exit(1)
			}
			images = append(images, *imageContentPtr)
		}
	}

	if *screenshotPtr {
		screenshots, err := utils.TakeScreenshots()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not generate screenshots: %v\n", err)
			os.Exit(1)
		}
		for _, fileName := range screenshots {
			imageContentPtr, err := utils.GetImageContent(fileName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not generate base64 encoding for screenshot %s: %v\n", fileName, err)
				os.Exit(1)
			}
			images = append(images, *imageContentPtr)
		}
	}

	// TODO: for some specific prompts we may not want to do this...
	// Read query from stdin
	queryString, timeoutError := readStdinWithTimeout(*timeoutPtr)
	if timeoutError != nil {
		fmt.Fprintln(os.Stderr, timeoutError)
		os.Exit(1)
	}

	// Append additional prompt if requested
	if *promptPtr != "" {
		queryString += *promptPtr
	}

	// Look in cache if specified
	if *cachePtr {
		if cachedResponse, err := cache.Get(queryString); err == nil {
			fmt.Println(cachedResponse)
			os.Exit(0)
		}
	}

	// flight check!
	// this makes sure the model that we are using can produce the output we want
	needsImageOutput := len(images) > 0
	validModel, reason := model.FlightCheck(needsImageOutput, false, false)
	if !validModel {
		fmt.Fprintf(os.Stderr, "Model %s cannot be used for your query: %s\n", model.ModelId, reason)
		os.Exit(1)
		//suggestedModel, err := models.SuggestedModel(needsImageOutput, false, false)
		//if err != nil {
		//	fmt.Fprintf(os.Stderr, "Could not find model to use")
		//	os.Exit(1)
		//}
		//fmt.Fprintf(os.Stderr, fmt.Sprintf("Using model %s\n", suggestedModel.ModelId))
		//model = suggestedModel
	}

	// create the query object
	var query *models.Query
	query, err = model.MakeQuery(queryString, images...)

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

	// store in cache if --cache was defined
	if *cachePtr {
		if err := cache.Set(queryString, response); err != nil {
			fmt.Fprintln(os.Stderr, "Error writing to cache:", err)
		}
	}
}
