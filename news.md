# agno-golang - Improvements & New Features Changelog

## Framework: Toolkit Enhancements

### Hook System (Pre/Post Execution)

The toolkit now supports pre and post-execution hooks at two levels:

- **Toolkit-level**: `AddPreHook()` / `AddPostHook()` - run on every method call
- **Method-level**: via `WithMethodPreHook()` / `WithMethodPostHook()` - run only for a specific method

Pre-hooks can abort execution by returning an error. Post-hooks receive the result and error for logging, metrics, etc.

### Result Caching with TTL

Automatic per-method result caching with MD5-based argument hashing:

```go
tk.Cache = toolkit.CacheConfig{Enabled: true, TTL: 5 * time.Minute}
```

- `ClearCache()` to manually invalidate
- Automatic expiration based on TTL

### Tool Filtering (Include/Exclude)

Control which methods are exposed to the agent:

```go
emailTool.IncludeTools("SendEmail", "ListEmails")  // only these
emailTool.ExcludeTools("DeleteEmail", "DeleteMailbox")  // all except these
```

### RegisterWithOptions

Register methods with advanced configuration via functional options:

```go
tk.RegisterWithOptions("DeleteFile", "Delete a file.", t, t.DeleteFile, DeleteFileParams{},
    toolkit.WithConfirmation(),      // requires user confirmation
    toolkit.WithStopAfterCall(),     // stops the tool-calling loop after execution
    toolkit.WithMethodPreHook(hook), // method-specific hook
)
```

### ConnectableTool Interface

Optional interface for tools that require connection lifecycle management:

```go
type ConnectableTool interface {
    Tool
    Connect() error
    Close() error
}
```

Implemented by `PostgresTool` and `DuckDBTool`.

### Tests

16 unit tests covering: registration, execution, hooks (pre/post, toolkit/method level), caching (TTL, clear, disabled), filtering (include/exclude), confirmation flags, and schema generation.

---

## New Tools

### Search & Web

| Tool | Description | Methods |
|------|-------------|---------|
| **TavilyTool** | AI-optimized web search via Tavily API | `Search`, `Extract` |
| **SerpAPITool** | Multi-engine search results (Google, Bing, Yahoo, DuckDuckGo, Baidu, Yandex) | `Search` |
| **SerperTool** | Google search via Serper API | `Search`, `News`, `Images` |
| **FirecrawlTool** | Web scraping with JS rendering | `Scrape`, `Crawl`, `Map` |
| **Crawl4AITool** | Web crawling without API key | `Crawl` |
| **NewspaperTool** | News article extraction and parsing | `GetArticle` |

### Utilities

| Tool | Description | Methods |
|------|-------------|---------|
| **CalculatorTool** | Mathematical expression evaluation | `Calculate` |
| **SleepTool** | Execution pause (rate limiting, waits) | `Sleep` |

### Research

| Tool | Description | Methods |
|------|-------------|---------|
| **PubMedTool** | Scientific article search on PubMed | `Search` |
| **RedditTool** | Reddit post search and reading | `SearchPosts`, `GetTopPosts` |

### Google Suite

| Tool | Description | Methods |
|------|-------------|---------|
| **GmailTool** | Gmail via OAuth2 API | `SendEmail`, `ReadEmails`, `GetEmail`, `CreateDraft` |
| **GoogleSheetsTool** | Spreadsheet automation | `ReadSheet`, `WriteSheet`, `AppendSheet`, `CreateSpreadsheet` |
| **GoogleDriveTool** | Drive file management | `ListFiles`, `DownloadFile`, `CreateFolder`, `DeleteFile` |

### Productivity

| Tool | Description | Methods |
|------|-------------|---------|
| **JiraTool** | Jira issue tracking | `CreateIssue`, `GetIssue`, `SearchIssues`, `AddComment` |
| **NotionTool** | Notion pages and databases | `SearchPages`, `GetPage`, `CreatePage`, `QueryDatabase` |

### Communication

| Tool | Description | Methods |
|------|-------------|---------|
| **DiscordTool** | Discord bot | `SendMessage`, `GetMessages`, `AddReaction` |
| **TelegramTool** | Telegram bot | `SendMessage`, `SendPhoto`, `GetUpdates` |

### Data & AI

| Tool | Description | Methods |
|------|-------------|---------|
| **PostgresTool** | PostgreSQL operations with ConnectableTool | `Query`, `ListTables`, `DescribeTable` |
| **DuckDBTool** | DuckDB analytics SQL | `Query`, `LoadCSV`, `ExportCSV` |
| **DalleTool** | DALL-E image generation | `GenerateImage` |

---

## EmailTool: Full Refactoring + WatchEmails

### Bugs Fixed

1. **Incompatible method signatures** - All 12 methods used `(ctx Context, args map[string]interface{})` but the toolkit expects `(params TypedStruct) (interface{}, error)`. All refactored to the correct pattern.

2. **toolkit.Tool interface not implemented** - Missing `Execute(methodName string, input json.RawMessage)` method. Added.

3. **Constructor panics** - `NewEmailTool` used `panic()` on error. Now returns `(*EmailTool, error)`.

4. **Manual GetJSONSchema removed** - The toolkit generates schemas automatically via reflection.

5. **Destructive operations require confirmation** - `DeleteMailbox` and `DeleteEmail` now use `WithConfirmation()`.

### New Feature: WatchEmails

Method to monitor new emails and trigger actions based on content or sender:

```go
type WatchEmailsParams struct {
    Mailbox       string  // Mailbox to monitor (default: INBOX)
    SubjectFilter string  // Filter by subject (e.g., "invoice", "urgent")
    SenderFilter  string  // Filter by sender (e.g., "boss@company.com")
    SinceMinutes  int     // Emails from the last N minutes (default: 60)
    Limit         int     // Max emails to return (default: 10)
}
```

Returns:
- `has_matches` - boolean indicating if new emails were found
- `new_emails` - list with uid, subject, from, date, body_preview (500 chars)
- `checked_at` / `since` - check timestamps

### 13 Available Methods

| Method | Description |
|--------|-------------|
| `SendEmail` | Send plain text email |
| `SendHTMLEmail` | Send HTML email |
| `ListEmails` | List emails from a mailbox |
| `ReadEmail` | Read email by UID |
| `SearchEmails` | Full-text search |
| `WatchEmails` | Monitor new emails with filters |
| `ListMailboxes` | List mailboxes |
| `CreateMailbox` | Create mailbox |
| `DeleteMailbox` | Delete mailbox (requires confirmation) |
| `DeleteEmail` | Delete email (requires confirmation) |
| `MarkAsRead` | Mark as read |
| `MarkAsUnread` | Mark as unread |
| `MoveEmail` | Move between mailboxes |

### Cookbook: Watch Example

Full example in `cookbook/tools/email/watch_example.go` demonstrating:

1. **One-shot watch** - Single check with sender or subject filters
2. **Polling loop** - Continuous monitoring every 2 minutes with automated rules:
   - Email with "urgent" in subject -> auto-reply acknowledgment
   - Email from "boss" -> mark as read and summarize
   - Email with "unsubscribe" -> mark as read
   - Everything else -> report details

```bash
# Normal mode (general examples)
GMAIL_EMAIL=you@gmail.com GMAIL_APP_PASSWORD=xxxx go run ./cookbook/tools/email/

# Watch mode (continuous monitoring)
GMAIL_EMAIL=you@gmail.com GMAIL_APP_PASSWORD=xxxx go run ./cookbook/tools/email/ --watch
```

---

## Summary

- **21 new tools** across 8 categories
- **5 framework improvements** (hooks, caching, filtering, confirmation, connection lifecycle)
- **16 tests** in the toolkit
- **EmailTool refactored** with 13 working methods
- **WatchEmails** for email-triggered automation
