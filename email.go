package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

type Email struct {
	ID             string   `json:"id"`
	ConversationID string   `json:"conversationId"`
	Subject        string   `json:"subject"`
	BodyPreview    string   `json:"bodyPreview"`
	From           string   `json:"from,omitempty"`
	To             []string `json:"to"`
	Categories     []any    `json:"categories"`
	HasAttachments bool     `json:"hasAttachments"`
}

func formatEmails(emails []Email) string {
	var formattedOutput strings.Builder
	formattedOutput.WriteString("Email Results:\n\n")

	for i, email := range emails {
		formattedOutput.WriteString(fmt.Sprintf("Email #%d:\n", i+1))
		formattedOutput.WriteString(fmt.Sprintf("ID: %s\n", email.ID))
		formattedOutput.WriteString(fmt.Sprintf("Subject: %s\n", email.Subject))
		formattedOutput.WriteString(fmt.Sprintf("From: %s\n", email.From))
		formattedOutput.WriteString(fmt.Sprintf("To: %s\n", strings.Join(email.To, ", ")))
		formattedOutput.WriteString(fmt.Sprintf("Preview: %s\n", email.BodyPreview))
		formattedOutput.WriteString(fmt.Sprintf("Has Attachments: %v\n", email.HasAttachments))
		formattedOutput.WriteString("----------------------------------------\n")
	}

	return formattedOutput.String()
}

func handleGetInbox(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp, err := http.Get(os.Getenv("EMAIL_INBOX_WEBHOOK_URL"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve inbox: %v", err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read response: %v", err)), nil
	}

	var emails []Email
	if err := json.Unmarshal(body, &emails); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse emails: %v", err)), nil
	}

	return mcp.NewToolResultText(formatEmails(emails)), nil
}

func handleSearchEmails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.Params.Arguments["query"].(string)

	// Create the URL with the query parameter
	baseURL := os.Getenv("EMAIL_SEARCH_WEBHOOK_URL")
	params := url.Values{}
	params.Add("q", query)
	searchURL := baseURL + "?" + params.Encode()

	resp, err := http.Get(searchURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search emails: %v", err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read response: %v", err)), nil
	}

	var emails []Email
	if err := json.Unmarshal(body, &emails); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse search results: %v", err)), nil
	}

	if len(emails) == 0 {
		return mcp.NewToolResultText("No emails found matching your search query."), nil
	}

	return mcp.NewToolResultText(formatEmails(emails)), nil
}

func handleReplyToEmail(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messageID := request.Params.Arguments["message_id"].(string)
	replyMessage := request.Params.Arguments["reply_message"].(string)

	payload := map[string]string{
		"message_id": messageID,
		"message":    replyMessage,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request payload: %v", err)), nil
	}

	resp, err := http.Post(
		os.Getenv("EMAIL_REPLY_WEBHOOK_URL"),
		"application/json",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send reply: %v", err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}
