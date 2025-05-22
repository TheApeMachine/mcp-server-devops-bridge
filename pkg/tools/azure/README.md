# Azure DevOps Tools

This package provides tools for interacting with Azure DevOps through the MCP (Model-Centric Programming) framework.

## Environment Setup

The following environment variables must be set:

```bash
AZURE_DEVOPS_ORG=your-organization-name
AZDO_PAT=your-personal-access-token
AZURE_DEVOPS_PROJECT=your-project-name
```

## Available Tools

### Work Item Tool

The Work Item tool allows you to manage and query Azure DevOps work items (tasks, bugs, user stories, epics, etc.).

#### Operations

The tool supports the following operations:

- `get_help`: Get help and documentation about the tool
- `get_examples`: Get example WIQL queries and usage patterns
- `find_work_items`: Find work items by state and parent relationship (recommended for AI assistants)
- `list_fields`: List available fields without needing a work item ID
- `search`: Search for work items by text in titles and descriptions
- `get_states`: Get all available work item states
- `get_work_item_types`: Get all available work item types
- `find_orphaned_items`: Find work items without parent links
- `find_blocked_items`: Find work items marked as blocked
- `find_overdue_items`: Find work items with past due dates
- `find_sprint_items`: Find work items in the current sprint
- `query`: Search for work items using WIQL
- `get_details`: Get details of specific work items by IDs
- `create`: Create a new work item
- `update`: Update a work item field
- `manage_relations`: Add or remove relations between work items
- `get_related_work_items`: Get work items related to a specific item
- `add_comment`: Add a comment to a work item
- `get_comments`: Get comments for a work item
- `manage_tags`: Add or remove tags from a work item
- `get_tags`: Get tags for a work item
- `get_templates`: Get work item templates
- `create_from_template`: Create a work item from a template
- `add_attachment`: Add an attachment to a work item
- `get_attachments`: Get attachments for a work item
- `remove_attachment`: Remove an attachment from a work item

#### Example Usage

To use the Work Item tool, you must specify an operation and appropriate parameters.

##### Getting Help

```json
{
  "operation": "get_help"
}
```

For help on a specific operation:

```json
{
  "operation": "get_help",
  "operation": "query"
}
```

##### Finding Work Items

Find items in DOING or REVIEW states:

```json
{
  "operation": "find_work_items",
  "states": "DOING,REVIEW"
}
```

Find items without Epic parents:

```json
{
  "operation": "find_work_items",
  "states": "DOING,REVIEW",
  "has_parent": "false",
  "parent_type": "Epic"
}
```

##### Querying Work Items with WIQL

```json
{
  "operation": "query",
  "query": "SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'"
}
```

##### Getting Work Item Details

```json
{
  "operation": "get_details",
  "ids": "123,456,789"
}
```

### Wiki Tool

The Wiki tool allows you to manage Azure DevOps wiki pages.

#### Operations

- `manage_wiki_page`: Create or update a wiki page
- `get_wiki_page`: Get a wiki page's content
- `list_wiki_pages`: List wiki pages
- `search_wiki`: Search for content in the wiki

### Sprint Tool

The Sprint tool allows you to manage Azure DevOps sprints/iterations.

#### Operations

- `get_current_sprint`: Get the current sprint
- `get_sprints`: Get all sprints

## Using with AI Assistants

When using these tools with AI assistants like Claude, start with one of these approaches:

1. Use the `get_help` operation to understand available operations:

   ```json
   {
     "operation": "get_help"
   }
   ```

2. Use the `find_work_items` operation to find relevant work items:

   ```json
   {
     "operation": "find_work_items",
     "states": "DOING,REVIEW"
   }
   ```

3. Use the `list_fields` operation to understand available fields:

   ```json
   {
     "operation": "list_fields"
   }
   ```

4. Search for work items by text:

   ```json
   {
     "operation": "search",
     "search_text": "authentication issue"
   }
   ```

5. Find items in the current sprint:
   ```json
   {
     "operation": "find_sprint_items"
   }
   ```
6. Create different types of work items:

   ```json
   {
     "operation": "create",
     "type": "User Story",
     "title": "As a user, I want to log in securely",
     "description": "Implement secure login functionality with proper error handling"
   }
   ```

   ```json
   {
     "operation": "create",
     "type": "Epic",
     "title": "Authentication System Overhaul",
     "description": "Redesign the entire authentication system"
   }
   ```

7. Get available states for proper filtering:
   ```json
   {
     "operation": "get_states"
   }
   ```

The tools include detailed help and examples to make them easier to use with AI assistants.
