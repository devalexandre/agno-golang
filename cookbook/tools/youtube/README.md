# YouTube Tool Example

This example demonstrates how to use the **YouTubeTool** to enable your agent to search for videos on YouTube.

## Prerequisites
- A Google Cloud Project
- [YouTube Data API v3](https://developers.google.com/youtube/v3) enabled
- An API Key

## Environment Variables
You must set the following environment variable:

```bash
export GOOGLE_API_KEY="your-api-key"
```

## Usage

```bash
go run main.go
```

## How it works
The agent uses the `YouTubeTool` to query the YouTube Data API. It retrieves video titles, channel names, publication dates, and links, allowing the agent to recommend videos to the user.
