package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func sendToWebhook(content string) {
	if config.Webhook == "" {
		return
	}

	logger.Info("Notifying webhook...")

	payload, err := json.Marshal(map[string]string{"content": content})

	if err != nil {
		logger.Fatalf("Failed to marshall code: %v", err)
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", config.Webhook, bytes.NewBuffer(payload))

	if err != nil {
		logger.Errorf("Failed to notify webhook when making request: %v", err)
		return
	}

	request.Header.Set("Content-Type", "application/json")

	res, err := client.Do(request)

	if err != nil {
		logger.Errorf("Failed to notify webhook when completing request: %v", err)
		return
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		logger.Errorf("Failed to decode body: %v", err)
		return
	}

	if res.StatusCode == 204 {
		logger.Info("Successfully notified webhook.")
		return
	}

	if res.StatusCode == 401 {
		logger.Infof("Failed to notify webhook. Does it exist? Body: %v", string(body))
		return
	}

	if res.StatusCode == 400 {
		logger.Infof("Failed to notify webhook. Got bad request. Body: %v", string(body))
		return
	}
}
