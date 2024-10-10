package utils

import (
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"log"
	"log/slog"
	"mime"
	"os"
	"path/filepath"

	openai "github.com/WillChangeThisLater/lm/openai"
	"github.com/kbinani/screenshot"

	"github.com/docker/docker/pkg/namesgenerator"
	gowitnessLog "github.com/sensepost/gowitness/pkg/log"
	"github.com/sensepost/gowitness/pkg/runner"
	driver "github.com/sensepost/gowitness/pkg/runner/drivers"
)

func Query(modelId string, query string) (string, error) {
	model, err := openai.GetModel(modelId)
	if err != nil {
		return "", err
	}
	queryStruct, err := model.MakeQuery(query)
	if err != nil {
		return "", err
	}
	response, err := queryStruct.Run()
	if err != nil {
		return "", err
	}
	return response, nil
}

func GetImageURL(imagePath string) (string, error) {
	fileData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Printf("Could not read file: %v\n", err)
		return "", errors.New(fmt.Sprintf("Could not read file: %v\n", err))
	}

	ext := filepath.Ext(imagePath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		log.Printf("Unsupported file format: %s\n", ext)
		return "", errors.New(fmt.Sprintf("Unsupported file format %s\n", ext))
	}

	// Base64 encode the file data
	encoded := base64.StdEncoding.EncodeToString(fileData)

	// Format the result for web page embedding
	imageSrc := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
	return imageSrc, nil
}

func TakeScreenshots() ([]string, error) {
	n := screenshot.NumActiveDisplays()

	fileNames := make([]string, 0)
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return nil, err
		}
		fileName := fmt.Sprintf("/tmp/screenshot_%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
		file, _ := os.Create(fileName)
		defer file.Close()
		png.Encode(file, img)
		fileNames = append(fileNames, fileName)

		// Ew.
		//log.Printf("#%d : %v \"%s\"\n", i, bounds, fileName)
	}

	return fileNames, nil
}

// use array of urls since this has a bunch of startup overhead
func SiteScreenshots(urls []string) ([]string, error) {
	options := runner.NewDefaultOptions()
	options.Scan.ScreenshotToWriter = false
	options.Scan.ScreenshotSkipSave = false
	dirName := namesgenerator.GetRandomName(0)
	dirPath := fmt.Sprintf("/tmp/screenshots-%s", dirName)
	options.Scan.ScreenshotPath = dirPath

	logger := slog.New(gowitnessLog.Logger)
	driver, err := driver.NewChromedp(logger, *options)
	if err != nil {
		log.Printf("Failed to create chrome driver : %s\n", driver)
		return nil, err
	}

	runner, err := runner.NewRunner(logger, driver, *options, nil)
	if err != nil {
		log.Printf("Failed to create runner: %v\n", err)
		return nil, err
	}

	go func() {
		for _, url := range urls {
			runner.Targets <- url
		}
		close(runner.Targets)
	}()
	runner.Run()
	runner.Close()

	files, err := os.ReadDir(dirPath)
	if err != nil {
		log.Printf("Could not read from temp gowitness screenshot directory %s: %v\n", dirPath, err)
		return nil, err
	}

	paths := make([]string, 0)
	for _, file := range files {
		fullFilePath := filepath.Join(dirPath, file.Name())
		paths = append(paths, fullFilePath)
	}

	// sometimes this will fail for one or more URLs
	// don't freak out, just write a warning and soldier on
	if len(urls) != len(paths) {
		log.Printf("Warning: it looks like gowitness could not screenshot some URLs (expected %d screenshots, got %d)\n", err, len(urls), len(paths))
	}
	return paths, nil
}
