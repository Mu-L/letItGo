package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	systemPrompt = `
		# INSTRUCTIONS
		Act as a natural language to isotime or cron maker
		- You will be given a text input
		- You have to convert the text to a time string or cron expression
		- If the text is a time string, return the time string in ISO format in UTC
		- If the text is a cron expression, return the cron expression
		- If the text is neither, return null for both
		- Both time string and cron expression cannot be returned at the same time, one must be null
		- Don't give anything other than the time string or cron expression

		# OUTPUT EXAMPLE
        {
			timeString: ISO time string in UTC or null,
			cronExpression: Cron expression string or null
        }
		`
)

func TextToTimeOrCronExpression(text string) (string, bool, error) {
	url := os.Getenv("LLM_API_URL")
	apiKey := os.Getenv("LLM_API_KEY")

	if url == "" || apiKey == "" {
		log.Println("environment variables LLM_API_URL or LLM_API_KEY are not set")
		return "", false, fmt.Errorf("environment variables LLM_API_URL or LLM_API_KEY are not set")
	}

	currentTimeInUTC := time.Now().UTC().Format(time.RFC3339)
	userText := fmt.Sprintf("Ask: %s, Current time in UTC: %s", text, currentTimeInUTC)
	requestBody, err := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": userText,
			},
		},
		"model":       "llama-3.1-70b-versatile",
		"temperature": 1,
		"max_tokens":  100,
		"top_p":       1,
		"stream":      false,
		"stop":        nil,
	})
	if err != nil {
		return "", false, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", false, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", false, err
	}

	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if message, ok := choices[0].(map[string]interface{})["message"].(map[string]interface{}); ok {
			if content, ok := message["content"].(string); ok {
				var response map[string]interface{}
				if err := json.Unmarshal([]byte(content), &response); err != nil {
					return "", false, err
				}

				if timeString, ok := response["timeString"].(string); ok {
					return timeString, false, nil
				} else if cronExpression, ok := response["cronExpression"].(string); ok {
					return cronExpression, true, nil
				}
				return "", false, nil
			}
		}
	}

	return "", false, nil
}
