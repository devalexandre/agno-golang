package utils

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// PrettyPrintMap takes a map and returns it formatted as a nice string
func PrettyPrintMap(data map[string]interface{}) string {
	var sb strings.Builder
	for k, v := range data {
		sb.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}
	return sb.String()
}

type SmartFlushBuffer struct {
	builder    strings.Builder
	lastFlush  time.Time
	flushDelay time.Duration
	flushFunc  func(ctx context.Context, data []byte) error
}

func NewSmartFlushBuffer(flushDelay time.Duration, flushFunc func(ctx context.Context, data []byte) error) *SmartFlushBuffer {
	return &SmartFlushBuffer{
		lastFlush:  time.Now(),
		flushDelay: flushDelay,
		flushFunc:  flushFunc,
	}
}

func (b *SmartFlushBuffer) Write(ctx context.Context, content string) {
	b.builder.WriteString(content)

	shouldFlush := time.Since(b.lastFlush) > b.flushDelay ||
		strings.HasSuffix(content, ".") ||
		strings.HasSuffix(content, "?") ||
		strings.HasSuffix(content, "!") ||
		strings.HasSuffix(content, "\n")

	if shouldFlush {
		_ = b.Flush(ctx)
	}
}

func (b *SmartFlushBuffer) Flush(ctx context.Context) error {
	if b.builder.Len() == 0 {
		return nil
	}
	data := b.builder.String()
	b.builder.Reset()
	b.lastFlush = time.Now()
	return b.flushFunc(ctx, []byte(data))
}
