# WebTool - Comprehensive Web Access Tool

A powerful web tool for the Agno Framework that provides HTTP request capabilities and web scraping functionality.

## Features

- **HTTP Requests**: Make GET, POST, PUT, DELETE requests to any URL
- **Web Scraping**: Extract content using CSS selectors
- **Page Analysis**: Get page titles, text content, and structured data
- **Custom Headers**: Support for custom HTTP headers
- **Timeout Control**: Configurable request timeouts
- **Error Handling**: Robust error handling with detailed responses

## Methods

### 1. HttpRequest
Make HTTP requests with full control over method, headers, and body.

**Parameters:**
- `url` (required): The URL to make the request to
- `method` (optional): HTTP method (GET, POST, PUT, DELETE). Default: GET
- `headers` (optional): Map of HTTP headers
- `body` (optional): Request body for POST/PUT requests
- `timeout` (optional): Request timeout in seconds. Default: 30

**Example:**
```go
// The AI can call this method like:
agent.PrintResponse("Make a POST request to https://httpbin.org/post with JSON data", false, true)
```

### 2. ScrapeContent
Extract specific content from web pages using CSS selectors.

**Parameters:**
- `url` (required): The URL to scrape
- `selector` (optional): CSS selector to extract specific elements
- `timeout` (optional): Request timeout in seconds. Default: 30

**Example:**
```go
// Extract all links from a page
agent.PrintResponse("Extract all navigation links from https://example.com using CSS selector 'nav a'", false, true)
```

### 3. GetPageText
Extract all text content from a web page, removing HTML tags.

**Parameters:**
- `url` (required): The URL to extract text from
- `timeout` (optional): Request timeout in seconds. Default: 30

**Example:**
```go
agent.PrintResponse("Get all text content from https://example.com and summarize it", false, true)
```

### 4. GetPageTitle
Get just the title of a web page.

**Parameters:**
- `url` (required): The URL to get the title from
- `timeout` (optional): Request timeout in seconds. Default: 30

**Example:**
```go
agent.PrintResponse("What is the title of https://golang.org?", false, true)
```

## Usage Examples

### Basic Setup

```go
package main

import (
    "context"
    "os"
    
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models"
    "github.com/devalexandre/agno-golang/agno/models/openai/chat"
    "github.com/devalexandre/agno-golang/agno/tools"
    "github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
    // Setup your model (OpenAI example)
    apiKey := os.Getenv("OPENAI_API_KEY")
    optsClient := []models.OptionClient{
        models.WithID("gpt-4o-mini"),
        models.WithAPIKey(apiKey),
    }
    
    chatOpenai, _ := chat.NewOpenAIChat(optsClient...)
    
    // Create WebTool
    webTool := tools.NewWebTool()
    
    // Create agent with WebTool
    agent := agent.NewAgent(agent.AgentConfig{
        Context: context.Background(),
        Model:   chatOpenai,
        Tools: []toolkit.Tool{
            webTool,
        },
        Markdown: true,
        ShowToolsCall: true,
    })
    
    // Use the WebTool
    agent.PrintResponse("Get the title from https://example.com", false, true)
}
```

### Advanced Usage Examples

1. **API Testing**:
   ```go
   agent.PrintResponse("Make a GET request to https://httpbin.org/json and show me the response", false, true)
   ```

2. **Web Scraping**:
   ```go
   agent.PrintResponse("Extract all headings from https://news.ycombinator.com using selector 'a.storylink'", false, true)
   ```

3. **Content Analysis**:
   ```go
   agent.PrintResponse("Get the main content from https://golang.org and provide a summary", false, true)
   ```

4. **Custom HTTP Headers**:
   ```go
   agent.PrintResponse("Make a request to https://httpbin.org/headers with a custom User-Agent header", false, true)
   ```

## Response Format

The WebTool returns structured responses with the following information:

### HttpRequest Response
```json
{
  "status_code": 200,
  "headers": {
    "content-type": "application/json"
  },
  "body": "response content",
  "url": "https://example.com",
  "success": true
}
```

### ScrapeContent Response
```json
{
  "url": "https://example.com",
  "selector": "h1",
  "elements": [
    {
      "text": "Main Heading",
      "href": "/link" // if applicable
    }
  ],
  "count": 1
}
```

### GetPageText/GetPageTitle Response
```json
{
  "url": "https://example.com",
  "title": "Example Domain",
  "text": "Full page content..."
}
```

## Error Handling

The WebTool includes comprehensive error handling:

- **Network errors**: Connection timeouts, DNS failures
- **HTTP errors**: 404, 500, etc.
- **Parse errors**: Invalid HTML, CSS selector issues
- **Validation errors**: Missing required parameters

All errors are returned in a structured format that the AI can understand and explain to users.

## Best Practices

1. **Be Respectful**: Add delays between requests to avoid overwhelming servers
2. **Use Appropriate Methods**: Choose the right method for your needs
3. **Handle Large Responses**: Content is automatically truncated to avoid excessive token usage
4. **Timeout Management**: Set appropriate timeouts for different types of requests
5. **CSS Selectors**: Use specific selectors to get exactly the content you need

## Running Examples

### OpenAI Examples
```bash
# Simple example
go run ./examples/openai/web_simple/main.go

# Advanced examples
go run ./examples/openai/web_advanced/main.go
```

### Ollama Examples
```bash
# Simple example  
go run ./examples/ollama/web_simple/main.go

# Advanced examples
go run ./examples/ollama/web_advanced/main.go
```

## Dependencies

- `github.com/PuerkitoBio/goquery` - For HTML parsing and CSS selectors
- Standard Go HTTP client for web requests

## Integration

The WebTool follows the Agno Framework toolkit interface and can be easily integrated with any agent configuration. It works seamlessly with both OpenAI and Ollama models.
