package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

// HackerNewsStory represents a story from Hacker News
type HackerNewsStory struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Score int    `json:"score"`
	By    string `json:"by"`
}

// HackerNewsTool is a custom tool for fetching Hacker News stories
type HackerNewsTool struct {
	toolkit.Toolkit
}

// GetTopStoriesParams defines the parameters for getting top stories
type GetTopStoriesParams struct {
	NumStories int `json:"num_stories" description:"Number of stories to fetch (default: 5)"`
}

// GetTopStories fetches top stories from Hacker News
func (hn *HackerNewsTool) GetTopStories(params GetTopStoriesParams) (string, error) {
	if params.NumStories == 0 {
		params.NumStories = 5
	}

	// Fetch top story IDs
	resp, err := http.Get("https://hacker-news.firebaseio.com/v0/topstories.json")
	if err != nil {
		return "", fmt.Errorf("failed to fetch story IDs: %w", err)
	}
	defer resp.Body.Close()

	var storyIDs []int
	if err := json.NewDecoder(resp.Body).Decode(&storyIDs); err != nil {
		return "", fmt.Errorf("failed to decode story IDs: %w", err)
	}

	// Fetch details for top N stories
	stories := make([]HackerNewsStory, 0, params.NumStories)
	for i := 0; i < params.NumStories && i < len(storyIDs); i++ {
		storyResp, err := http.Get(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", storyIDs[i]))
		if err != nil {
			continue
		}

		var story HackerNewsStory
		if err := json.NewDecoder(storyResp.Body).Decode(&story); err != nil {
			storyResp.Body.Close()
			continue
		}
		storyResp.Body.Close()

		stories = append(stories, story)
	}

	// Format as readable text
	var result string
	for i, story := range stories {
		result += fmt.Sprintf("%d. %s\n", i+1, story.Title)
		result += fmt.Sprintf("   URL: %s\n", story.URL)
		result += fmt.Sprintf("   Score: %d | By: %s\n\n", story.Score, story.By)
	}

	return result, nil
}

func main() {
	ctx := context.Background()

	// Enable markdown
	utils.SetMarkdownMode(true)

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create custom Hacker News tool
	hnTool := &HackerNewsTool{
		Toolkit: toolkit.NewToolkit(),
	}
	hnTool.Name = "hackernews"
	hnTool.Description = "Fetch top stories from Hacker News"

	// Register the GetTopStories method
	hnTool.Register(
		"GetTopStories",
		"Get the top stories from Hacker News",
		hnTool,
		hnTool.GetTopStories,
		GetTopStoriesParams{},
	)

	// Create a Tech News Reporter Agent with custom Hacker News tool
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "TechReporter",
		Instructions: `You are a tech-savvy Hacker News reporter with a passion for all things technology! 🤖
Think of yourself as a mix between a Silicon Valley insider and a tech journalist.

Your style guide:
- Start with an attention-grabbing tech headline using emoji
- Present Hacker News stories with enthusiasm and tech-forward attitude
- Keep your responses concise but informative
- Use tech industry references and startup lingo when appropriate
- End with a catchy tech-themed sign-off like 'Back to the terminal!' or 'Pushing to production!'

Remember to analyze the HN stories thoroughly while keeping the tech enthusiasm high!`,
		Tools: []toolkit.Tool{hnTool},
		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Example usage
	ag.PrintResponse("Summarize the top 5 stories on Hacker News", true, true)

	// More example prompts:
	/*
		ag.PrintResponse("What are the trending tech discussions on HN right now?", true, true)
		ag.PrintResponse("What's the most upvoted story today?", true, true)
		ag.PrintResponse("Tell me about the top 3 AI-related stories on HN", true, true)
	*/
}
