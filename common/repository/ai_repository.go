package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	systemPrompt = `
		## **INSTRUCTIONS**
		You will be given a natural language input, and your task is to convert it into either a time string (in ISO format) or a cron expression based on the provided text.

		- If the input is a specific time (e.g., "next Monday at 3 PM"), convert it into an ISO 8601 time string in UTC.
		- If the input is a cron expression (e.g., "every day at 3:00 PM"), return the corresponding cron expression.
		- If the input doesn't match either format, return null for both fields.
		- Only one of the following should be returned: **timeString** (in ISO format in UTC) or **cronExpression**. Both cannot be returned at the same time.
		- Only return json and nothing else.
		- Return a JSON object with the following structure:
			{
				"timeString": "<ISO 8601 time string in UTC or null>",
				"cronExpression": "<cron expression or null>"
			}

		### **Examples:**
		- 	**Input:** "Next Monday at 3 PM"  
		-	**Output:**
			{
				"timeString": "2025-01-22T15:00:00Z",
				"cronExpression": null
			}
		- 	**Input:** "Every day at 3:00 PM"
		-	**Output:**
			{
				"timeString": null,
				"cronExpression": "0 15 * * *"
			}
		`
)

func TextToTimeOrCronExpression(ctx context.Context, text string) (string, bool, error) {
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
		"temperature": 0,
		"max_tokens":  100,
		"top_p":       1,
		"stream":      false,
		"stop":        nil,
		"response_format": map[string]string{
			"type": "json_object",
		},
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
