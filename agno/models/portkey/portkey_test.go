package portkey

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

func TestDefaults(t *testing.T) {
	rec := presettest.Install(t)
	t.Setenv(EnvAPIKey, "pk-secret")
	t.Setenv(EnvVirtualKey, "vk-1")

	m, err := NewPortkeyChat(models.WithClientParams(map[string]interface{}{
		ParamConfig: map[string]interface{}{"retry": map[string]interface{}{"attempts": 3}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.GetID() != DefaultModel {
		t.Fatalf("GetID = %q", m.GetID())
	}
	req := presettest.Invoke(t, rec, m)
	if req.URL != DefaultBaseURL+"/chat/completions" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.Header.Get("x-portkey-api-key") != "pk-secret" {
		t.Fatalf("x-portkey-api-key = %q", req.Header.Get("x-portkey-api-key"))
	}
	if req.Header.Get("x-portkey-virtual-key") != "vk-1" {
		t.Fatalf("x-portkey-virtual-key = %q", req.Header.Get("x-portkey-virtual-key"))
	}
	if req.Header.Get("x-portkey-config") != `{"retry":{"attempts":3}}` {
		t.Fatalf("x-portkey-config = %q", req.Header.Get("x-portkey-config"))
	}
}

func TestMissingKey(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	if _, err := NewPortkeyChat(); err == nil {
		t.Fatal("expected error")
	}
}
