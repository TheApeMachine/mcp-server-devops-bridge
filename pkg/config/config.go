// Package config provides centralized configuration management for the MCP Server MultiTool.
package config

import (
	"fmt"
	"os"
	"strings"
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

	// OpenAI configuration
	OpenAI struct {
		APIKey string
		Model  string
	}

	// Sentry configuration
	Sentry struct {
		DSN                string
		AuthToken          string
		Organization       string
		DefaultProjectSlug string
		ProjectIDs         []string
		ProjectNames       []string
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

		// OpenAI
		config.OpenAI.APIKey = os.Getenv("OPENAI_API_KEY")
		config.OpenAI.Model = os.Getenv("OPENAI_MODEL")
		if config.OpenAI.Model == "" {
			config.OpenAI.Model = v.GetString("openai.model")
		}

		// Sentry
		config.Sentry.DSN = os.Getenv("SENTRY_DSN")
		config.Sentry.AuthToken = os.Getenv("SENTRY_AUTH_TOKEN")
		config.Sentry.Organization = os.Getenv("SENTRY_ORG")
		config.Sentry.DefaultProjectSlug = os.Getenv("SENTRY_PROJECT_SLUG")
		config.Sentry.ProjectIDs = strings.Split(os.Getenv("SENTRY_PROJECT_IDS"), ",")
		config.Sentry.ProjectNames = strings.Split(os.Getenv("SENTRY_PROJECT_NAMES"), ",")

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
