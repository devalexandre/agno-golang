package weaviate

import "github.com/google/uuid"

// weaviateNamespace is a fixed namespace for deriving deterministic UUIDv5s.
var weaviateNamespace = uuid.MustParse("6f9619ff-8b86-d011-b42d-00cf4fc964ff")

// toUUID returns id unchanged if it is already a valid UUID, otherwise a
// deterministic UUIDv5 derived from it (Weaviate requires UUID object IDs).
func toUUID(id string) string {
	if id == "" {
		return uuid.NewString()
	}
	if _, err := uuid.Parse(id); err == nil {
		return id
	}
	return uuid.NewSHA1(weaviateNamespace, []byte(id)).String()
}
