package utils

import (
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
