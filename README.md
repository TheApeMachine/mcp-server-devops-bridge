# 🚀 MCP Server Multi Tools 🌉

> [!NOTE]
> **Model Context Protocol (MCP) Bridge** - A powerful service that connects AI agents with DevOps and communication tools through a standardized interface.

Welcome to the **MCP Server Multi Tools**! This service acts as a seamless integration layer, enabling AI agents and systems to interact with services like **Azure DevOps**, **Slack**, **GitHub**, and even **other agents (with their own tools)** using a simple, unified protocol.

---

## 🛠️ Available Tools & Providers

### 📊 **DevOps & Project Management**

A comprehensive suite for total project management:

- **📝 Work Items**: Create, read, update, search, and comment
- **🏃‍♂️ Sprints**: Manage sprints, view contents, and track progress  
- **🔍 WIQL**: Execute custom Work Item Query Language statements
- **🔗 Enrichment**: Augment work items with GitHub, Slack, and Sentry context
- **📁 Git Content**: Fetch file content from associated repositories

### 🐙 GitHub Integration

- Seamless version control and pull request workflows
- Repository content access and management

### 💬 **Communication**

- Real-time notifications and messaging
- Channel posting for agent communication
- Team collaboration enhancement

### 🤖 **Advanced Agent System**

> [!IMPORTANT]
> **Agents-in-Agents**: Revolutionary nested agent architecture

**Your AI agent can create and manage other agents**, each with:

- 🐋 **Docker container** with full Debian Linux system
- 🌐 **Web browser** capabilities  
- 🔄 **Iterative work** processes
- 💬 **Inter-agent communication**

---

## 🚀 Quick Start Guide

### 📋 Prerequisites

> [!WARNING]
> Ensure you have the following before proceeding:

- ✅ [Go](https://go.dev/doc/install) (latest version)
- 🔑 Service credentials (Azure DevOps, Slack tokens, etc.)
- 🐳 Docker Desktop (for agents-in-agents feature)

### Step 1️⃣: **Clone & Build**

```bash
# Clone the repository
git clone https://github.com/theapemachine/mcp-server-multi-tools.git
cd mcp-server-devops-bridge

# Build the server
go build -o mcp-server-multi-tools .
```

### Step 2️⃣: **Environment Setup**

```bash
# Copy and configure environment variables
cp start.sh.example start.sh
vim start.sh  # Add your credentials
```

> [!TIP]
> The server is configured entirely through environment variables for maximum flexibility.

### Step 3️⃣: **MCP Client Configuration**

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "multi-tools": {
      "command": "/path/to/mcp-server-multi-tools/start.sh",
      "args": []
    }
  }
}
```

---

## 🎬 **See It In Action**

> [!NOTE]
> **Real AI Development Workflow** - Watch Claude Sonnet 4 leverage the bridge to understand requirements, manage tasks, and write code.

![AI Agent Development Example](./agents.png)

*The agent uses MCP tools to orchestrate complex development workflows through a single, unified interface.*

For a complete history of testing, including the upgraded communication system and the headless browser tool, see [agents-full.pdf](./agents-full.pdf).

---

## 🤝 Contributing

We welcome contributions! Here's how to get involved:

### 🔄 **Contribution Workflow**

1. **🍴 Fork** the repository
2. **🌿 Branch** your feature (`git checkout -b feature/AmazingFeature`)  
3. **💾 Commit** your changes (`git commit -m 'Add AmazingFeature'`)
4. **🚀 Push** to branch (`git push origin feature/AmazingFeature`)
5. **📬 Open** a Pull Request

### 💡 **What We're Looking For**

- 🔧 New tool integrations
- 📚 Documentation improvements  
- 🐛 Bug fixes and optimizations
- 🎨 UI/UX enhancements

---

## 📜 License

**MIT License** - See [LICENSE](LICENSE) for details

---

*Built with 🤷 for the developer community*
