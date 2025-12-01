# ChromaDB Example with Testcontainers

This example demonstrates how to use **ChromaDB** as a vector database with Agno, using **Testcontainers** to automatically spin up a ChromaDB instance for testing/development.

## Prerequisites
- Docker (must be running)
- Ollama (running locally with `nomic-embed-text` model)

## Features
- Automatic ChromaDB container management
- Document embedding with Ollama
- Vector search
- Metadata filtering

## Usage

```bash
go run main.go
```

## How it works
1.  Uses `testcontainers-go` to pull and run the `chromadb/chroma` Docker image.
2.  Connects the `ChromaDB` vector store implementation to the container.
3.  Embeds documents using `OllamaEmbedder`.
4.  Performs semantic search.
5.  Cleans up the container automatically on exit.
