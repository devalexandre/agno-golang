---
name: summarize
description: "Summarize or extract text from URLs, articles, PDFs, and YouTube videos. Use when asked to summarize a link or transcribe a video."
metadata:
  version: "1.0.0"
  requires:
    bins: ["curl"]
---

# Summarize Skill

Techniques for summarizing content from various sources.

## When to Use

Use this skill when the user asks:
- "What's this link/video about?"
- "Summarize this URL/article"
- "Extract text from this page"
- "Transcribe this YouTube video"

## Web Pages

### Using curl + text extraction

```bash
# Fetch and extract readable text
curl -s "https://example.com" | sed 's/<[^>]*>//g' | tr -s ' \n'
```

### Using readability-cli (if available)

```bash
readable "https://example.com" --output text
```

## PDFs

### Using pdftotext (poppler)

```bash
pdftotext input.pdf - | head -500
```

### Using Python

```python
import PyPDF2
reader = PyPDF2.PdfReader("input.pdf")
for page in reader.pages:
    print(page.extract_text())
```

## YouTube (best-effort)

### Using yt-dlp for subtitles

```bash
# Download auto-generated subtitles
yt-dlp --write-auto-sub --sub-lang en --skip-download -o "/tmp/%(id)s" "https://youtu.be/VIDEO_ID"
cat /tmp/VIDEO_ID.en.vtt
```

### Without yt-dlp

Use the YouTube transcript API or fetch the page and extract from JSON data.

## Summarization Approach

1. Extract raw text from the source
2. If text is very long (>5000 words), chunk it into sections
3. Summarize each chunk, then create an overall summary
4. Return a structured summary with:
   - Title/Source
   - Key points (3-5 bullet points)
   - Detailed summary (if requested)

## Tips

- For very long content, summarize first, then offer to expand on specific sections
- Always cite the source URL
- If the content is behind a paywall or login, inform the user
