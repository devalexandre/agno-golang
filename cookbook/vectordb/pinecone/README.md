# Pinecone Example

This example demonstrates how to use **Pinecone** as a vector database with Agno.

## Prerequisites
- A Pinecone account and API Key
- A pre-created Index in Pinecone (dimension must match your embedder, e.g., 768 for `nomic-embed-text`)
- Ollama running locally

## Environment Variables
You must set the following environment variables:

```bash
export PINECONE_API_KEY="your-api-key"
export PINECONE_INDEX_URL="https://your-index-host.svc.pinecone.io"
```

## Usage

```bash
go run main.go
```

## How it works
1.  Connects to your Pinecone index using the REST API.
2.  Embeds documents using `OllamaEmbedder`.
3.  Upserts vectors with metadata to Pinecone.
4.  Performs semantic search to retrieve relevant documents.
