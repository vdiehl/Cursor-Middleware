package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const targetURL = "http://localhost:8000/v1/chat/completions" // adjust as needed

// OriginalTool describes a tool from the incoming JSON
type OriginalTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type OriginalContent struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	Id           string                 `json:"id"`
	Input        map[string]interface{} `json:"input"`
	Name         string                 `json:"name"`
	CacheControl string                 `json:"cache_control"`
	Content      []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content"`
}

type TransformedContent struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	Id           string                 `json:"id"`
	Input        map[string]interface{} `json:"input"`
	CacheControl string                 `json:"cache_control"`
	Name         string                 `json:"name"`
}

// TransformedTool matches the OpenAI-style function format
type TransformedTool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function is the nested part of TransformedTool
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type OriginalMessage struct {
	Content []OriginalContent `json:"content"`
	Role    string            `json:"role"`
}

type TransformedMessage struct {
	Content []TransformedContent `json:"content"`
	Role    string               `json:"role"`
}

type TransformedToolResult struct {
	Type         string `json:"type"`
	Text         string `json:"text"`
	CacheControl string `json:"cache_control"`
	ToolUseId    string `json:"tool_use_id"`
}

// ProxyPayload is the general incoming request
type ProxyPayload struct {
	Tools      []OriginalTool         `json:"tools"`
	ToolChoice map[string]interface{} `json:"tool_choice"`
	Other      map[string]interface{} `json:"-"`
	Messages   []OriginalMessage      `json:"messages"`
}

// TransformedPayload for output
type TransformedPayload struct {
	Tools      []TransformedTool      `json:"tools"`
	ToolChoice interface{}            `json:"tool_choice,omitempty"`
	Other      map[string]interface{} `json:"-"`
	Messages   []TransformedMessage   `json:"messages"`
}

// Custom unmarshal to capture additional fields
func (p *ProxyPayload) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract known fields
	toolsRaw, _ := raw["tools"]
	toolChoiceRaw, _ := raw["tool_choice"]
	messagesRaw, _ := raw["messages"]
	delete(raw, "messages")
	delete(raw, "tools")
	delete(raw, "tool_choice")

	// Marshal/unmarshal known fields into their types
	toolsBytes, _ := json.Marshal(toolsRaw)
	json.Unmarshal(toolsBytes, &p.Tools)

	messagesBytes, _ := json.Marshal(messagesRaw)
	json.Unmarshal(messagesBytes, &p.Messages)

	if toolChoiceRaw != nil {
		if m, ok := toolChoiceRaw.(map[string]interface{}); ok {
			p.ToolChoice = m
		}
	}

	// Preserve other unknown fields
	p.Other = raw
	return nil
}

func translateTool(t OriginalTool) TransformedTool {
	return TransformedTool{
		Type: "function",
		Function: Function{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		},
	}
}

func translateMessage(m OriginalMessage) TransformedMessage {
	if m.Content[0].Type == "tool_result" {
		return TransformedMessage{
			Content: []TransformedContent{
				{
					Type:         "text",
					Text:         m.Content[0].Content[0].Text,
					Id:           m.Content[0].Id,
					Input:        m.Content[0].Input,
					CacheControl: m.Content[0].CacheControl,
					Name:         m.Content[0].Name,
				},
			},
			Role: "tool",
		}
	}

	if m.Content[0].Type == "tool_use" {
		return TransformedMessage{
			Content: []TransformedContent{
				{
					Type:         "text",
					Text:         m.Content[0].Content[0].Text,
					Id:           m.Content[0].Id,
					Input:        m.Content[0].Input,
					CacheControl: m.Content[0].CacheControl,
					Name:         m.Content[0].Name,
				},
			},
			Role: "tool",
		}
	}

	return TransformedMessage{
		Content: []TransformedContent{
			{
				Type:         m.Content[0].Type,
				Text:         m.Content[0].Text,
				Id:           m.Content[0].Id,
				Input:        m.Content[0].Input,
				CacheControl: m.Content[0].CacheControl,
				Name:         m.Content[0].Name,
			},
		},
		Role: m.Role,
	}
}

// ConvertToolTypesToText replaces all "type":"tool_use" and "type":"tool_result" with "type":"text"
// and returns the updated map[string]interface{}
func ConvertToolTypesToText(input map[string]interface{}) (map[string]interface{}, error) {
	// Marshal to JSON string
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	jsonStr := string(jsonBytes)

	//fmt.Println(jsonStr)
	// Replace specific "type" values
	jsonStr = strings.ReplaceAll(jsonStr, `"tool_use"`, `"text", "text":"incorrect tool use"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"tool_result"`, `"text", "text":"incorrect tool result"`)
	// jsonStr = strings.ReplaceAll(jsonStr, `"stream": true`, `"stream": false`)
	// jsonStr = strings.ReplaceAll(jsonStr, `"stream":true`, `"stream":false`)

	//fmt.Println(jsonStr)
	// Unmarshal back into map
	var output map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &output)
	if err != nil {
		return nil, errors.New("failed to parse updated JSON")
	}

	return output, nil
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST supported", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var payload ProxyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Translate tools
	transformedTools := make([]TransformedTool, 0, len(payload.Tools))
	for _, t := range payload.Tools {
		transformedTools = append(transformedTools, translateTool(t))
	}

	// Translate messages
	transformedMessages := make([]TransformedMessage, 0, len(payload.Messages))
	for _, m := range payload.Messages {
		transformedMessages = append(transformedMessages, translateMessage(m))
	}

	// Rebuild final payload
	result := map[string]interface{}{
		"tools":    transformedTools,
		"messages": transformedMessages,
	}
	if payload.ToolChoice != nil {
		if choiceType, ok := payload.ToolChoice["type"]; ok {
			if strType, ok := choiceType.(string); ok {
				result["tool_choice"] = strType
			}
		}
	}

	for k, v := range payload.Other {
		result[k] = v // preserve extra fields
	}

	result, err = ConvertToolTypesToText(result)
	if err != nil {
		http.Error(w, "Failed to convert tool types to text: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Send to target system
	resultBytes, _ := json.Marshal(result)
	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(resultBytes))
	if err != nil {
		http.Error(w, "Failed to forward request: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	http.HandleFunc("/v1/chat/completions", proxyHandler)
	log.Println("Proxy listening on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
