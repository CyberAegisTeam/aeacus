//go:build !phocus

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type studioEvaluation struct {
	Score   int                      `json:"score"`
	Results []studioEvaluationResult `json:"results"`
}

type studioEvaluationResult struct {
	ID      string `json:"id"`
	Index   int    `json:"index"`
	Message string `json:"message"`
	Points  int    `json:"points"`
	Status  string `json:"status"`
}

func studioEvaluateData() studioEvaluation {
	quietEnabled = true
	readConfig()
	scoreChecks()
	passing := map[int]bool{}
	for _, item := range image.Points {
		passing[item.Index] = true
	}
	for _, item := range image.Penalties {
		passing[item.Index] = true
	}
	result := studioEvaluation{Score: image.Score, Results: make([]studioEvaluationResult, 0, len(conf.Check))}
	for index, check := range conf.Check {
		message := check.Message
		_ = deobfuscateData(&message)
		status := "failing"
		if passing[index+1] {
			status = "passing"
		}
		result.Results = append(result.Results, studioEvaluationResult{ID: check.ID, Index: index + 1, Message: message, Points: check.Points, Status: status})
	}
	quietEnabled = false
	return result
}

func studioEvaluate() error {
	return json.NewEncoder(os.Stdout).Encode(studioEvaluateData())
}

func studioRelease() error {
	result := studioEvaluateData()
	if err := validateStudioReleaseScore(result); err != nil {
		return err
	}
	releaseImage()
	return nil
}

func validateStudioReleaseScore(result studioEvaluation) error {
	if result.Score == 0 {
		return nil
	}
	message := fmt.Sprintf("release blocked: current score is %d; the image must start at exactly 0", result.Score)
	for _, item := range result.Results {
		if item.Status == "passing" {
			message += fmt.Sprintf("; %s (%d points)", item.Message, item.Points)
		}
	}
	return errors.New(message)
}

func studioVerifyData(path string) error {
	if path == "" {
		return errors.New("scoring.dat path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	plain, err := decryptConfig(string(data))
	if err != nil {
		return err
	}
	var parsed config
	if _, err := toml.Decode(plain, &parsed); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"valid": true, "name": parsed.Name, "title": parsed.Title, "checks": len(parsed.Check), "version": parsed.Version})
}
