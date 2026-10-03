package serve

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"reasonix/internal/assembly/boot"
	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/config"
	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
)

func TestMCPPreviewSeparatesDisplayFromInstallValues(t *testing.T) {
	root := testenv.TempDir(t)
	home := testenv.TempDir(t)
	t.Setenv("REASONIX_HOME", home)
	t.Chdir(root)
	resolver := &provider.StaticResolver{Descriptors: []provider.Descriptor{{Ref: "neutral/fixture", Model: "fixture", Default: true}}, Providers: map[string]provider.Provider{"neutral/fixture": credentialPreviewProvider{}}}
	ctrl, err := boot.Build(t.Context(), boot.Options{Home: home, WorkspaceRoot: root, Model: "neutral/fixture", ProviderResolver: resolver, SessionDir: filepath.Join(root, "sessions"), TokenMode: boot.TokenModeFull, Sink: event.Discard})
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	input := `{"mcpServers":{"neutral":{"url":"https://host/mcp?%74oken=fixture-secret","env":{"PASSWORD":"fixture-secret"},"headers":{"Authorization":"fixture-secret"}}}}`
	body, _ := json.Marshal(map[string]string{"input": input})
	w := httptest.NewRecorder()
	New(ctrl, NewBroadcaster(), config.ServeConfig{}).mcpParse(w, httptest.NewRequest("POST", "/mcp/parse", strings.NewReader(string(body))))
	var response struct {
		Servers []map[string]any `json:"servers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Servers) != 1 {
		t.Fatalf("response: %s", w.Body.String())
	}
	s := response.Servers[0]
	if !strings.Contains(s["url"].(string), "fixture-secret") {
		t.Fatal("install URL lost")
	}
	for _, key := range []string{"displayUrl", "displayEnv", "displayHeaders"} {
		value, ok := s[key]
		if !ok {
			t.Errorf("missing %s", key)
			continue
		}
		b, _ := json.Marshal(value)
		if strings.Contains(string(b), "fixture-secret") {
			t.Errorf("%s leaked", key)
		}
	}
}

type credentialPreviewProvider struct{}

func (credentialPreviewProvider) Name() string { return "credential-preview" }
func (credentialPreviewProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	chunks := make(chan provider.Chunk)
	close(chunks)
	return chunks, nil
}
