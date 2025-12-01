# Google Search Tool Example

This example demonstrates how to use the **GoogleSearchTool** to enable your agent to search the web using the official Google Custom Search JSON API.

## Prerequisites
- A Google Cloud Project
- [Custom Search API](https://developers.google.com/custom-search/v1/overview) enabled
- An API Key
- A Custom Search Engine ID (CX)

## Environment Variables
You must set the following environment variables:

```bash
export GOOGLE_API_KEY="your-api-key"
export GOOGLE_CX="your-cx-id"
```

## Usage

```bash
go run main.go
```

## How it works
The agent uses the `GoogleSearchTool` to query Google. It retrieves search results including titles, snippets, and links, which the agent uses to answer the user's question.
