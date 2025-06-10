# 🚀 MCP Server Multi Tools 🌉

Welcome to the **MCP Server Multi Tools**! This powerful service acts as a seamless integration layer, connecting a standardized **Model Context Protocol (MCP)** interface with a wide array of essential DevOps and communication tools.

It enables AI agents and other systems to interact with services like **Azure DevOps**, **Slack**, **GitHub**, and even **Other Agents** using a simple, unified protocol. This bridge empowers developers to build sophisticated automation and AI-driven workflows without getting bogged down by the complexities of individual service APIs.

## ✨ Features

The MCP server comes packed with a rich set of tools, organized into providers:

### DevOps & Productivity

- **Azure DevOps**: A comprehensive suite of tools for total project management.
  - **Work Items**: Create, read, update, search, and comment on work items.
  - **Sprints**: Manage sprints, view their contents, and get overviews of progress.
  - **WIQL**: Execute custom Work Item Query Language statements.
  - **Enrichment**: Augment work items with context from GitHub, Slack, and Sentry.
  - **Git Content**: Fetch file content directly from associated GitHub repositories.

- **GitHub**:
  - Seamlessly integrate with your version control and pull request workflows.

### Communication

- **Slack**:
  - Post messages to any channel, enabling real-time notifications and agent communication.

### Extensible Agent System

- **Agents-in-Agents** Have your AI agent (MCP client) create its own agents, and manage them. Each agent has access to a docker container running a Debian Linux system, a web browser, is able to work iteratively, and can communicate with other agents.

## 🛠️ Getting Started

### Prerequisites

- [Go](https://go.dev/doc/install) (latest version recommended)
- Access tokens and credentials for the services you want to integrate (Azure DevOps, Slack, etc.).
- Docker (Desktop) installed and running (for the agents-in-agents feature).

### 1. Clone the Repository

```bash
git clone https://github.com/theapemachine/mcp-server-multi-tools.git
cd mcp-server-devops-bridge
```

Don't forget to actually build the server.

```bash
go build -o mcp-server-multi-tools .
```

### 2. Configure Environment Variables

The server is configured entirely through environment variables. Copy the example script and fill in your credentials.

```bash
cp start.sh.example start.sh
vim start.sh # Or use your favorite editor
```

### 3. Add MCP Configuration

The example below is how to add the MCP server to Claude Desktop.

```json
"mcpServers": {
    "multi-tools": {
        "command": "/path/to/mcp-server-multi-tools/start.sh",
        "args": [],
    }
}
```

## 🤖 Example in Action: AI-Powered Development

Here's a glimpse of an AI agent (Claude Sonnet 4) leveraging the bridge to develop a Python program. The agent uses the provided tools to understand requirements, manage tasks, and write code, all orchestrated through the MCP server.

![agents.png](./agents.png)

## 🤝 Contributing

Contributions are welcome! Whether it's adding a new tool, improving documentation, or fixing a bug, please feel free to open a pull request.

1. **Fork** the repository.
2. Create your **feature branch** (`git checkout -b feature/AmazingFeature`).
3. **Commit** your changes (`git commit -m 'Add some AmazingFeature'`).
4. **Push** to the branch (`git push origin feature/AmazingFeature`).
5. Open a **Pull Request**.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
