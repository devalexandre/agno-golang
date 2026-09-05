package mongodb

import (
	"context"
	"testing"
)

func TestRequiresURI(t *testing.T) {
	if _, err := New(context.Background(), Options{}); err == nil {
		t.Fatal("expected error when neither URI nor Client provided")
	}
}
