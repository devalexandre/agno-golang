package knowledge

import (
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// IsValidFileFormat verifica se o arquivo tem um formato válido
func IsValidFileFormat(filePath string, validFormats []string) bool {
	if len(validFormats) == 0 {
		return true // Se não há restrições, aceitar todos
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	for _, format := range validFormats {
		if strings.ToLower(format) == ext {
			return true
		}
	}

	return false
}

// GetFileExtension retorna a extensão do arquivo
func GetFileExtension(filePath string) string {
	return strings.ToLower(filepath.Ext(filePath))
}

// ConvertDocumentPointers converte []*document.Document para []document.Document
func ConvertDocumentPointers(docs []*document.Document) []document.Document {
	if docs == nil {
		return nil
	}

	result := make([]document.Document, len(docs))
	for i, doc := range docs {
		if doc != nil {
			result[i] = *doc
		}
	}

	return result
}

// ConvertDocuments converte []document.Document para []*document.Document
func ConvertDocuments(docs []document.Document) []*document.Document {
	if docs == nil {
		return nil
	}

	result := make([]*document.Document, len(docs))
	for i := range docs {
		result[i] = &docs[i]
	}

	return result
}
