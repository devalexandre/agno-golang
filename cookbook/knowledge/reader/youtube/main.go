// YouTubeReader: fetch a video's transcript (no API key; uses YouTube's
// public watch page + innertube player endpoint, like youtube_transcript_api).
//
// Requires network access. Usage:
//
//	go run ./cookbook/knowledge/reader/youtube [video-url-or-id] [lang...]
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	video := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	if len(os.Args) > 1 {
		video = os.Args[1]
	}
	r := reader.NewYouTubeReader()
	if len(os.Args) > 2 {
		r.Languages = os.Args[2:] // preference order; manual captions beat auto-generated
	}
	docs, err := r.Read(context.Background(), video)
	if err != nil {
		log.Fatal(err)
	}
	d := docs[0]
	content := d.Content
	if len(content) > 500 {
		content = content[:500] + "..."
	}
	fmt.Printf("%s (lang=%v, %d chars)\n%s\n", d.ID, d.Metadata["language"], len(d.Content), content)
}
