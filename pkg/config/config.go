// Package config provides centralized configuration management for the MCP Server MultiTool.
package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/viper"
)

// Config holds the complete configuration for the application
type Config struct {
	// Azure DevOps configuration
	Azure struct {
		OrganizationURL     string
		PersonalAccessToken string
		Project             string
	}

	// GitHub configuration
	GitHub struct {
		PersonalAccessToken string
		Organization        string
	}

	// Slack configuration
	Slack struct {
		BotToken       string
		UserToken      string
		DefaultChannel string
	}

	// Memory system configuration
	Memory struct {
		// Vector store (Qdrant)
		Qdrant struct {
			URL    string
			APIKey string
		}

		// Graph database (Neo4j)
		Neo4j struct {
			URL      string
			Username string
			Password string
		}
	}

	// OpenAI configuration
	OpenAI struct {
		APIKey string
		Model  string
	}

	// Email configuration
	Email struct {
		InboxWebhookURL  string
		SearchWebhookURL string
		ReplyWebhookURL  string
	}

	// Sentry configuration
	Sentry struct {
		DSN                string
		AuthToken          string
		Organization       string
		DefaultProjectSlug string
	}
}

var (
	once   sync.Once
	config *Config
)

// Load initializes and loads the configuration from environment variables
func Load() *Config {
	once.Do(func() {
		v := viper.New()

		// Set default values
		v.SetDefault("openai.model", "gpt-4o-mini")

		// Load from environment variables
		v.AutomaticEnv()

		// Map environment variables to config structure
		config = &Config{}

		// Azure DevOps
		config.Azure.OrganizationURL = "https://dev.azure.com/" + os.Getenv("AZURE_DEVOPS_ORG")
		config.Azure.PersonalAccessToken = os.Getenv("AZDO_PAT")
		config.Azure.Project = os.Getenv("AZURE_DEVOPS_PROJECT")

		// GitHub
		config.GitHub.PersonalAccessToken = os.Getenv("GITHUB_PAT")
		config.GitHub.Organization = os.Getenv("GITHUB_ORG")

		// Slack
		config.Slack.BotToken = os.Getenv("SLACK_BOT_TOKEN")
		config.Slack.UserToken = os.Getenv("SLACK_USER_TOKEN")
		config.Slack.DefaultChannel = os.Getenv("DEFAULT_SLACK_CHANNEL")

		// Memory - Qdrant
		config.Memory.Qdrant.URL = os.Getenv("QDRANT_URL")
		config.Memory.Qdrant.APIKey = os.Getenv("QDRANT_API_KEY")

		// Memory - Neo4j
		config.Memory.Neo4j.URL = os.Getenv("NEO4J_URL")
		config.Memory.Neo4j.Username = os.Getenv("NEO4J_USER")
		config.Memory.Neo4j.Password = os.Getenv("NEO4J_PASSWORD")

		// OpenAI
		config.OpenAI.APIKey = os.Getenv("OPENAI_API_KEY")
		config.OpenAI.Model = os.Getenv("OPENAI_MODEL")
		if config.OpenAI.Model == "" {
			config.OpenAI.Model = v.GetString("openai.model")
		}

		// Email
		config.Email.InboxWebhookURL = os.Getenv("EMAIL_INBOX_WEBHOOK_URL")
		config.Email.SearchWebhookURL = os.Getenv("EMAIL_SEARCH_WEBHOOK_URL")
		config.Email.ReplyWebhookURL = os.Getenv("EMAIL_REPLY_WEBHOOK_URL")

		// Sentry
		config.Sentry.DSN = os.Getenv("SENTRY_DSN")
		config.Sentry.AuthToken = os.Getenv("SENTRY_AUTH_TOKEN")
		config.Sentry.Organization = os.Getenv("SENTRY_ORG")
		config.Sentry.DefaultProjectSlug = os.Getenv("SENTRY_PROJECT_SLUG")

		config.GitHub.PersonalAccessToken = os.Getenv("GITHUB_PAT")
		config.GitHub.Organization = os.Getenv("GITHUB_ORG")
	})

	return config
}

// Validate checks if all required configuration values are set
func (c *Config) Validate() error {
	// List of validation errors
	var errors []string

	// Check critical configurations
	if c.Azure.OrganizationURL == "https://dev.azure.com/" || c.Azure.PersonalAccessToken == "" || c.Azure.Project == "" {
		errors = append(errors, "Azure DevOps configuration is incomplete")
	}

	// For the memory system, check if we have at least one of Qdrant or Neo4j configured
	if (c.Memory.Qdrant.URL == "" || c.Memory.Qdrant.APIKey == "") &&
		(c.Memory.Neo4j.URL == "" || c.Memory.Neo4j.Username == "" || c.Memory.Neo4j.Password == "") {
		errors = append(errors, "At least one memory system (Qdrant or Neo4j) must be configured")
	}

	// Check if OpenAI API key is set for agent system
	if c.OpenAI.APIKey == "" {
		errors = append(errors, "OpenAI API key is required for agent functionality")
	}

	// If any errors were found, return them as a combined error
	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %v", errors)
	}

	return nil
}
