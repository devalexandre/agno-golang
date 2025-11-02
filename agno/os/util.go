package os

import (
	"fmt"
)

func StringPtr(s string) *string {
	return &s
}

// getKnowledgeInstanceByDBID returns a knowledge instance by DB ID
// This matches Python's get_knowledge_instance_by_db_id behavior:
// - If no db_id provided and only 1 instance exists: return that instance
// - If no db_id provided and multiple instances exist: return error 400
// - If db_id provided: find matching instance or return error 404
func getKnowledgeInstanceByDBID(instances []*KnowledgeInstance, dbID string) (*KnowledgeInstance, error) {
	// No db_id provided
	if dbID == "" {
		// If only one instance, return it
		if len(instances) == 1 {
			return instances[0], nil
		}
		// Multiple instances require db_id
		if len(instances) > 1 {
			return nil, fmt.Errorf("multiple knowledge instances found, db_id parameter required")
		}
		// No instances at all
		return nil, fmt.Errorf("no knowledge instances available")
	}

	// db_id provided, find matching instance
	for _, instance := range instances {
		if instance.DBID == dbID {
			return instance, nil
		}
	}

	// Not found
	return nil, fmt.Errorf("knowledge instance with db_id '%s' not found", dbID)
}
