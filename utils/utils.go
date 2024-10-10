package utils

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"

	openai "github.com/WillChangeThisLater/lm/openai"
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
