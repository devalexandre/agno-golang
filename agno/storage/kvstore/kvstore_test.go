package kvstore

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

func TestMemoryBackendConformance(t *testing.T) {
	storagetest.RunDBConformance(t, New(NewMemoryBackend()))
	storagetest.RunSessionSearchConformance(t, New(NewMemoryBackend()))
	storagetest.RunMetricsConformance(t, New(NewMemoryBackend()))
	storagetest.RunKnowledgeConformance(t, New(NewMemoryBackend()))
	storagetest.RunStorageConformance(t, New(NewMemoryBackend()))
}
