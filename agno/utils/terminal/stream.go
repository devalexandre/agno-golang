package terminal

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// StreamingPanel handles progressive rendering of streaming content
type StreamingPanel struct {
	renderer   *PanelRenderer
	buffer     strings.Builder
	lastUpdate time.Time
	mu         sync.Mutex
	minDelay   time.Duration // Minimum delay between updates
}

// NewStreamingPanel creates a new streaming panel handler
func NewStreamingPanel(renderer *PanelRenderer) *StreamingPanel {
	return &StreamingPanel{
		renderer:   renderer,
		lastUpdate: time.Now(),
		minDelay:   50 * time.Millisecond, // Update at most every 50ms
	}
}

// Update adds a chunk of content to the buffer and renders if appropriate
func (s *StreamingPanel) Update(chunk string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer.WriteString(chunk)

	// Check if we should render based on timing and content
	now := time.Now()
	shouldRender := false

	// Render if enough time has passed
	if now.Sub(s.lastUpdate) >= s.minDelay {
		shouldRender = true
	}

	// Render if we hit a sentence boundary
	if strings.ContainsAny(chunk, ".!?") {
		shouldRender = true
	}

	// Render if buffer is getting large
	if s.buffer.Len() > 100 {
		shouldRender = true
	}

	if shouldRender {
		s.render()
		s.lastUpdate = now
	}
}

// render displays the current buffer content
func (s *StreamingPanel) render() {
	content := s.buffer.String()
	if content == "" {
		return
	}

	// Clear previous line and render new content
	// Use ANSI escape codes to move cursor up and clear
	fmt.Print("\r\033[K") // Clear current line
	fmt.Print(content)
}

// Flush forces a render of any remaining buffered content
func (s *StreamingPanel) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.buffer.Len() > 0 {
		s.render()
	}
}

// Clear resets the buffer
func (s *StreamingPanel) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer.Reset()
}

// GetContent returns the current buffered content
func (s *StreamingPanel) GetContent() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buffer.String()
}

// SetMinDelay sets the minimum delay between renders
func (s *StreamingPanel) SetMinDelay(delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.minDelay = delay
}
