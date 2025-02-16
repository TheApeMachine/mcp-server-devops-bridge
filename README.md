# MCP Server DevOps Bridge

A Model Context Protocol (MCP) server that enables Claude to bridge and orchestrate your DevOps toolchain through natural language. Stop context-switching between tools - let Claude handle the integration seamlessly.

## 🌉 Bridge Your DevOps Tools

Connect your essential DevOps platforms:

- **Azure DevOps** - Work items, wiki, sprints
- **GitHub** - Pull requests, code reviews, repositories
- **Slack** - Team communication, notifications, updates

## 🎯 Key Benefits

- **Natural Language Interface** - No need to learn different APIs or CLI tools
- **Cross-Platform Integration** - Work items link to PRs, PRs trigger notifications
- **Unified Workflow** - Let Claude handle the context switching between tools
- **Flexible Architecture** - Easy to extend with new integrations

## 🚀 Getting Started

### Prerequisites

- Go 1.23 or later
- Access tokens for your platforms:
  - Azure DevOps PAT
  - GitHub PAT (optional)
  - Slack Bot Token (optional)

### Installation

1. Clone and build:

```bash
git clone https://github.com/yourusername/mcp-server-devops-bridge
cd mcp-server-devops-bridge
go build
```

2. Configure your environment:

```bash
export AZURE_DEVOPS_ORG="your-org"
export AZDO_PAT="your-pat-token"
export AZURE_DEVOPS_PROJECT="your-project"

# Optional integrations
export GITHUB_PAT="your-github-pat"
export SLACK_BOT_TOKEN="your-slack-token"
export DEFAULT_SLACK_CHANNEL="some-slack-channel-id"
```

3. Add to Claude's configuration:

```json
{
  "mcpServers": {
    "wcgw": {
      "command": "uv",
      "args": [
        "tool",
        "run",
        "--from",
        "wcgw@latest",
        "--python",
        "3.12",
        "wcgw_mcp"
      ]
    },
    "devops-bridge": {
      "command": "/full/path/to/mcp-server-devops-bridge/mcp-server-devops-bridge",
      "args": [],
      "env": {
        "AZURE_DEVOPS_ORG": "organization",
        "AZDO_PAT": "personal_access_token",
        "AZURE_DEVOPS_PROJECT": "project",
        "SLACK_DEFAULT_CHANNEL": "channel_id",
        "SLACK_BOT_TOKEN": "bot_token",
        "GITHUB_PAT": "personal_access_token"
      }
    }
  }
}
```

> In this configuration I added wcgw_mcp, because this will technically allow you to ask Claude (or which ever client you use) to directly fix your code as well, but this is untested at the moment. In any case it will also allow you to ask Claude to check you code-bases and use it as context when creating work items in Azure DevOps.

## 💡 Example Workflows

### Cross-Platform Task Management

```txt
"Create a user story for the new authentication feature, link it to the existing GitHub PR #123, and notify the team in Slack"
```

### Code Review Workflow

```txt
"Find all work items related to authentication, show their linked PRs, and summarize recent code review comments"
```

### Status Reporting

```txt
"Generate a sprint report including:
- Work item status from Azure Boards
- PR review status from GitHub
- Team discussions from Slack"
```

### Documentation Management

```txt
"Update the wiki page for authentication and link it to relevant work items and PRs"
```

## 🔌 Supported Integrations

### Azure DevOps

- Work item management (any field supported)
- Wiki documentation
- Sprint planning
- Attachments and discussions

### GitHub

- PR management and reviews
- Code search and navigation
- Repository management
- Review comments

### Slack

- Rich message formatting
- Thread management
- Notifications
- Team coordination

## 🛠 Architecture

The bridge uses the Model Context Protocol to provide Claude with structured access to your DevOps tools. This enables:

- Type-safe operations
- Proper error handling
- Clear feedback
- Extensible design

## 🤝 Contributing

We welcome contributions! Key areas for enhancement:

- Additional platform integrations (GitLab, Jira, etc.)
- Enhanced cross-platform workflows
- Improved reporting capabilities
- New integration patterns

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🔒 Security

- Store access tokens securely
- Grant minimal required permissions
- Regularly rotate credentials
- Audit integration access

## 🆘 Support

- Open an issue for bugs or feature requests
- Check discussions for common questions
- Review wiki for implementation guides
