# MCP Server DevOps Bridge 🚀

> Connect your DevOps tools with the power of AI

This project evolved from a simple experiment into a powerful bridge between your essential DevOps platforms, providing a unified interface through Claude and other AI assistants. With integrated agent capabilities, it can even delegate work autonomously!

## 🌉 Bridge Your DevOps Tools

Connect your essential DevOps platforms with a natural language interface:

- **Azure DevOps** - Work items, wiki, sprints, and project management
- **GitHub** - Pull requests, code reviews, repositories
- **Slack** - Team communication, notifications, updates
- **Browser Automation** - Web interactions, screenshots, JavaScript execution
- **AI Agents** - Delegate tasks to autonomous AI agents

## 🎯 Key Benefits

- **Natural Language Interface** - Interact with your tools using plain English
- **Cross-Platform Integration** - Work items link to PRs, PRs trigger notifications
- **Unified Workflow** - Let AI handle the context switching between tools
- **Flexible Architecture** - Easy to extend with new integrations
- **Autonomous Workflows** - Create AI agents to handle repetitive tasks

## 🚀 Getting Started

### Prerequisites

- Go 1.23.4 or later
- Docker (required for agent system)
- Access tokens for your platforms:
  - Azure DevOps PAT
  - GitHub PAT (optional)
  - Slack Bot Token (optional)
  - OpenAI API Key (for agents)

### Installation

1. Clone and build:

```bash
git clone https://github.com/theapemachine/mcp-server-devops-bridge
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

# AI and Memory integrations
export OPENAI_API_KEY="your-api-key"
export QDRANT_URL="http://localhost:6333"
export QDRANT_API_KEY="your-qdrant-api-key"
export NEO4J_URL="http://localhost:7474"
export NEO4J_USER="neo4j"
export NEO4J_PASSWORD="your-neo4j-password"
```

3. Add to Claude's configuration:

```json
{
  "mcpServers": {
    "devops-bridge": {
      "command": "/full/path/to/mcp-server-devops-bridge/mcp-server-devops-bridge",
      "args": [],
      "env": {
        "AZURE_DEVOPS_ORG": "organization",
        "AZDO_PAT": "personal_access_token",
        "AZURE_DEVOPS_PROJECT": "project",
        "SLACK_DEFAULT_CHANNEL": "channel_id",
        "SLACK_BOT_TOKEN": "bot_token",
        "GITHUB_PAT": "personal_access_token",
        "OPENAI_API_KEY": "openaikey",
        "QDRANT_URL": "http://localhost:6333",
        "QDRANT_API_KEY": "yourkey",
        "NEO4J_URL": "yourneo4jinstance",
        "NEO4J_USER": "neo4j",
        "NEO4J_PASSWORD": "neo4jpassword"
      }
    }
  }
}
```

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

### Autonomous Agent Workflow

```txt
"Create an agent to monitor our authentication PRs, summarize code changes, and post daily updates to Slack"
```

## 🔌 Key Features

### Agents System

The project includes a powerful agent system built on OpenAI's GPT-4o-mini, enabling Claude to create its own long-running agents that can:

- Execute tasks autonomously in secure Docker containers
- Communicate with other agents
- Access system tools and commands
- Process tasks in the background
- Run commands in isolated environments for security

Under the hood, each agent runs inside a dedicated Docker container, providing:

- Isolated execution environment
- Secure command execution
- Controlled access to host resources
- Clean separation between agents

### Azure DevOps Integration

- **Work Item Management**
  - Create, update, and query work items
  - Add/remove tags
  - Manage work item attachments
  - Add comments and track discussions
  - Create work items from templates
  - Manage work item relationships
- **Wiki Management**
  - Create and update wiki pages
  - Search wiki content
  - Retrieve page content and subpages
- **Sprint Management**
  - Query current and upcoming sprints
  - Track sprint progress

### New Modular Azure DevOps Tools

The Azure DevOps integration now includes a set of modular tools that are easier to use and more AI-friendly:

- **azure_sprint_items**: Find work items in the current sprint
- **azure_create_work_item**: Create new work items with rich options
- **azure_create_work_item_custom**: Create work items with support for custom fields
- **azure_find_items_by_status**: Search for work items by status and other criteria
- **azure_get_work_item**: Get detailed information about specific work items
- **azure_work_item_comments**: Add and retrieve comments on work items

These new tools feature:

- Focused single-responsibility design
- Clear parameter naming and validation
- Detailed error messages with suggestions
- Consistent response formats (text and JSON)
- AI-assistant friendly design

For more details, see `pkg/tools/azure/tools/README.md`

### GitHub Integration

- **Pull Request Management**
  - List open/closed pull requests
  - Get detailed PR information
  - Review and comment on PRs
- **Code Search**
  - Search across repositories
  - Filter by path, language, and repository

### Slack Integration

- **Message Formatting**
  - Format messages using Block Kit
  - Support for headers, sections, and context blocks
- **Message Search**
  - Search message history
  - Filter by channel and user
- **Message Posting**
  - Post messages to channels
  - Support for threaded replies
  - Rich message formatting with blocks

### Browser Automation

- **Web Navigation**
  - Open websites
  - Execute JavaScript
  - Take screenshots
  - Wait for elements
- **Form Filling**
  - Input text
  - Click buttons
  - Handle dropdowns
- **Data Extraction**
  - Scrape content
  - Process results

### Memory Management

- **Vector Storage (Qdrant)**
  - Semantic search capabilities
  - Document storage with metadata
  - Similarity search with configurable thresholds
- **Graph Database (Neo4j)**
  - Store relationships between memories
  - Query using Cypher
  - Track temporal relationships

### AI Integration

- **Claude Integration**
  - Direct chat capabilities
  - Memory-augmented conversations
  - Context-aware responses
- **OpenAI Integration**
  - GPT-4 integration for agents
  - Memory retrieval and formatting
  - Structured output generation

### Code Analysis

- Code complexity analysis
- Potential bug detection
- Security issue identification
- Context storage for future reference

### Cross-Integration Features

- **Status Report Generation**
  - Combine data from multiple sources
  - Sprint status reports
  - Work item summaries
  - PR status integration
- **Work Item Reminders**
  - Slack notifications
  - Customizable messages
  - Automated tracking
- **Standup Report Generation**
  - Team-based reporting
  - State-grouped work items
  - Rich Slack formatting

## 🛠 Architecture

The bridge uses the Model Context Protocol to provide Claude with structured access to your DevOps tools. This enables:

- Type-safe operations
- Proper error handling
- Clear feedback
- Extensible design

## 🔒 Security

- Store access tokens securely
- Grant minimal required permissions
- Regularly rotate credentials
- Audit integration access

## Alternative Setup: start.sh

If you don't have direct access to modify environment variables, create a `start.sh` script and make it executable:

```bash
#!/bin/bash

# Azure DevOps Configuration
export AZURE_DEVOPS_ORG="YOUR ORG"
export AZDO_PAT="YOUR PAT"
export AZURE_DEVOPS_PROJECT="YOUR PROJECT"

# GitHub Configuration
export GITHUB_PAT="YOUR PAT"

# Slack Configuration
export SLACK_BOT_TOKEN="YOUR TOKEN"
export DEFAULT_SLACK_CHANNEL="YOUR CHANNEL ID"

# OpenAI Configuration
export OPENAI_API_KEY="YOUR API KEY"

# Qdrant Configuration
export QDRANT_URL="http://localhost:6333"
export QDRANT_API_KEY="your-qdrant-api-key"

# Neo4j Configuration
export NEO4J_URL="http://localhost:7474"
export NEO4J_USER="neo4j"
export NEO4J_PASSWORD="your-neo4j-password"

# Email Configuration (if using email features)
export EMAIL_INBOX_WEBHOOK_URL="YOUR WEBHOOK URL"
export EMAIL_SEARCH_WEBHOOK_URL="YOUR WEBHOOK URL"
export EMAIL_REPLY_WEBHOOK_URL="YOUR WEBHOOK URL"

/path/to/mcp-server-devops-bridge/mcp-server-devops-bridge
```

## 🤝 Contributing

We welcome contributions! Key areas for enhancement:

- Additional platform integrations (GitLab, Jira, etc.)
- Enhanced cross-platform workflows
- Improved reporting capabilities
- New integration patterns

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🆘 Support

- Open an issue for bugs or feature requests
- Check discussions for common questions
- Review wiki for implementation guides

## 🧠 Memory System

The bridge implements an intelligent memory system that enables AI assistants to automatically:

1. **Retrieve relevant memories** before responding to queries
2. **Store important context** from interactions for future reference

### Memory Architecture

The memory system uses a dual-store approach:

- **Vector Storage (Qdrant)** - For semantic search of unstructured text
- **Graph Database (Neo4j)** - For entity relationships and structured queries

### Automatic Memory Integration

The system includes a middleware layer that enhances MCP tools with memory capabilities:

```go
// Apply memory middleware to any tool handler
wrappedHandler := MemoryMiddleware(originalHandler)
```

This middleware:

1. Automatically searches for relevant memories based on the tool and query
2. Injects found memories into the context before the response
3. Extracts and stores important information from interactions

### Memory Flow

1. **Query Phase**: Before processing a tool request

   - Extract query context from the tool parameters
   - Search vector and graph stores for relevant memories
   - Format memories for inclusion in the response

2. **Response Phase**: After processing a tool request
   - Analyze the response for important information
   - Use OpenAI to extract structured knowledge
   - Store in both vector and graph databases

### Benefits

- **Contextual Awareness**: AI can recall relevant facts from previous interactions
- **Knowledge Persistence**: Important information is automatically preserved
- **Cross-Session Memory**: Context is maintained between different conversations
- **Transparent Enhancement**: Memory injection is automatic and seamless

### Usage

Memory-enhancing specific tools is simple:

```go
// Create your tool
myTool := mcp.NewTool("my_tool", /* ... */);

// Original handler function
func handleMyTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // Tool implementation
}

// Wrap with memory middleware
wrappedHandler := MemoryMiddleware(handleMyTool)

// Register with MCP server
mcpServer.AddTool(myTool, wrappedHandler)
```

### Configuration

The memory system can be configured through environment variables:

```bash
# Vector Store (Qdrant)
export QDRANT_URL="http://localhost:6333"
export QDRANT_API_KEY="your-qdrant-api-key"

# Graph Database (Neo4j)
export NEO4J_URL="http://localhost:7474"
export NEO4J_USER="neo4j"
export NEO4J_PASSWORD="your-neo4j-password"

# OpenAI (for memory extraction)
export OPENAI_API_KEY="your-openai-key"
```

# MCP Server DevOps Bridge with Agent System

This repository contains the code for an MCP (Mission Control Panel) server DevOps bridge with a powerful agent system that allows AI models to create, manage, and coordinate long-running agents.

## Overview

The system allows AI models to:

1. Create new long-running agents with customized system prompts and tasks
2. Send commands to existing agents
3. Facilitate communication between agents through a messaging system
4. Monitor and manage agent lifecycles

## Key Components

### Agent Management

The system provides tools for:

- **Creating Agents**: Create new agents with custom system prompts and tasks
- **Listing Agents**: View all running agents and their status
- **Sending Commands**: Send instructions or queries to specific agents
- **Subscribing to Topics**: Have agents listen for messages on specific channels
- **Killing Agents**: Terminate agents when they're no longer needed

### Inter-Agent Communication

Agents can communicate with each other through:

- **Message Bus**: A central messaging system that routes messages to subscribed agents
- **Topics**: Channels that agents can publish to and subscribe to
- **Direct Commands**: Send direct instructions to specific agents

### System Tools

The system provides tools for:

- **Command Execution**: Run system commands with proper security controls
- **Messaging**: Send and receive messages between agents
- **Agent Management**: Create, monitor, and terminate agents

## Usage

### Creating and Coordinating Agents

```go
// Get all agent-related tools
tools := ai.GetAllToolsAsOpenAI()

// Create an OpenAI client
client := openai.NewClient()

// Use OpenAI to coordinate agents
messages := []openai.ChatCompletionMessageParamUnion{
    openai.SystemMessage(`You are a coordinator of AI agents.`),
    openai.UserMessage("Create two agents and have them work together."),
}

// Call OpenAI with our tools
params := openai.ChatCompletionNewParams{
    Model:    openai.F(openai.ChatModelGPT4o),
    Messages: openai.F(messages),
    Tools:    openai.F(tools),
}

// Process the response and handle tool calls
// ...
```

### Example Agent Workflow

1. **Create Agents**: Create specialized agents for different tasks

   ```
   Tool: agent
   Arguments: {"id": "researcher", "system_prompt": "You are a research agent...", "task": "Find information about climate change"}
   ```

2. **Subscribe to Topics**: Have agents listen for relevant messages

   ```
   Tool: subscribe_agent
   Arguments: {"agent_id": "writer", "topic": "research_results"}
   ```

3. **Send Messages**: Share information between agents

   ```
   Tool: send_agent_message
   Arguments: {"topic": "research_results", "content": "Here is the information I found..."}
   ```

4. **Send Commands**: Give direct instructions to agents
   ```
   Tool: send_command
   Arguments: {"agent_id": "writer", "command": "Summarize the research in 3 paragraphs"}
   ```

## Security

The system implements several security measures:

- Agents only have access to the commands and paths explicitly granted to them
- Command execution is containerized to provide isolation
- All communication is managed through controlled channels

## Getting Started

See the `examples/agent_example.go` file for a complete example of how to use the agent system to create and coordinate multiple agents.
