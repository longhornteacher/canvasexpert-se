# CE-SE MCP adapter

`cse mcp` runs an MCP server over stdio on the Windows computer that has the CE-SE workspace and saved Canvas credential. It reuses the Engine commands; it does not accept arbitrary shell commands or expose the Canvas token.

The adapter currently exposes these tools:

| Tool | Purpose |
|---|---|
| `engine_reference` | Read command help, file formats, field limits, or synthetic examples. |
| `list_courses` | List configured Canvas courses. |
| `list_modules` | List a configured course's Canvas modules. |
| `pull_assignment` | Pull an exact assignment and its edit baseline. |
| `pull_page` | Pull an exact Page and its edit baseline. |
| `pull_work` | Pull work through CE-SE, optionally scoped to a course and assignment. |
| `read_workspace_file` | Read `AGENTS.md`, `CLAUDE.md`, `context/`, `playbooks/`, or `courses/` files. Hidden paths, `.cse/`, and `engine/` are blocked. |
| `write_draft` | Create or revise `.md` / `.yaml` / `.yml` drafts under `courses/`. Replacing an existing file requires the SHA-256 returned by a read, to avoid overwriting changed unsent work. |
| `validate_document` | Run the Engine's local document validator for a file under `courses/`. |
| `start_review` | Start the existing loopback review page and return its URL and review ID. |
| `review_status` | Check for the teacher's approval or revision notes after they use that page. |

The adapter does not expose a general `cse` command, raw Canvas HTTP, student mapping files, or a direct MCP posting tool. For supported assignment scoring, creation and edits, `start_review` invokes the existing `cse review`; the teacher must use the page to approve the write. Open the returned `127.0.0.1` URL on the Windows computer running the adapter. The adapter's authoring scope is currently limited to workflows that can use that review flow; direct-write operations such as native Page creation, file uploads and module assembly are not exposed by MCP yet.

## Local run

Run from a configured workspace with its Engine installed:

```powershell
.\engine\cse.exe --workspace . mcp
```

The process uses stdin/stdout for MCP messages. Do not add logging to stdout. Keep the process attached to the MCP host; it can access the machine's saved Canvas credential only through the Engine's existing credential code.

## Claude Cowork

Add a local MCP server in Claude Desktop/Cowork's MCP or extension settings using:

- Command: the absolute path to `engine\cse.exe`
- Arguments: `--workspace`, the absolute path to the teacher workspace, `mcp`
- Transport: stdio

The workspace must be installed and configured on the same Windows computer. Give Cowork access to the workspace folder for the instruction and course files it needs. Keep the `.cse` folder excluded from its folder access where the host supports exclusions; the MCP server independently refuses hidden workspace paths.

## ChatGPT Work via Secure MCP Tunnel

ChatGPT does not connect directly to a local stdio server. OpenAI's Secure MCP Tunnel supports a private local server without a public inbound endpoint. Create a tunnel in OpenAI Platform, associate it with the ChatGPT context that will use it, then run `tunnel-client` on the CE-SE computer with its `--mcp-command` set to the absolute `cse.exe` path plus `--workspace <workspace> mcp`. Add the custom MCP server in ChatGPT and select the tunnel. Follow OpenAI's current [Secure MCP Tunnel guide](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels) for tunnel identity, API-key handling, permissions, and setup details.

OpenAI's current plan documentation says Pro can connect MCP servers with read/fetch permissions and that full MCP write support is for Business and Enterprise/Edu; Plus is not specified there. Check your account's actual developer-mode and custom-server options before relying on MCP access. Tunnel permissions and ChatGPT custom-server permissions are separate.

## Data and validation boundaries

Canvas work and draft contents returned by the tools enter the connected model's context. CE-SE keeps the student-name mapping in `.cse/`; it is neither readable through this adapter nor included in tool results. Teacher notes in explicitly requested workspace context may be returned to the model, so only connect an account approved for that data.

The adapter has been compile-checked locally. It has not yet been validated with Claude Cowork, ChatGPT Work, OpenAI's tunnel, a district environment, or Canvas Live through MCP. The existing CE-SE Canvas evidence does not prove MCP transport, tool discovery, or review-page use from those clients.
