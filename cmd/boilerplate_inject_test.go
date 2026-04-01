package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── stripJSONCComments ───────────────────────────────────────────────────────

func TestStripJSONCComments_RemovesLineComment(t *testing.T) {
	input := `{"key": "value"} // trailing comment`
	got := string(stripJSONCComments([]byte(input)))
	if strings.Contains(got, "//") {
		t.Errorf("stripJSONCComments() did not remove line comment: %q", got)
	}
	if !strings.Contains(got, `"key"`) {
		t.Errorf("stripJSONCComments() removed key: %q", got)
	}
}

func TestStripJSONCComments_RemovesBlockComment(t *testing.T) {
	input := `{"key": /* comment */ "value"}`
	got := string(stripJSONCComments([]byte(input)))
	if strings.Contains(got, "comment") {
		t.Errorf("stripJSONCComments() did not remove block comment: %q", got)
	}
}

func TestStripJSONCComments_PreservesURLInString(t *testing.T) {
	input := `{"url": "https://example.com/path"}`
	got := string(stripJSONCComments([]byte(input)))
	if !strings.Contains(got, "https://example.com/path") {
		t.Errorf("stripJSONCComments() stripped URL inside string: %q", got)
	}
}

func TestStripJSONCComments_PreservesSlashStarInString(t *testing.T) {
	input := `{"regex": "/* not a comment */"}`
	got := string(stripJSONCComments([]byte(input)))
	if !strings.Contains(got, "/* not a comment */") {
		t.Errorf("stripJSONCComments() stripped content inside string: %q", got)
	}
}

// ─── deepMergeMap ─────────────────────────────────────────────────────────────

func TestDeepMergeMap_AddsNewKeys(t *testing.T) {
	dst := map[string]interface{}{"a": "1"}
	src := map[string]interface{}{"b": "2"}
	result := deepMergeMap(dst, src)
	if result["a"] != "1" {
		t.Error("deepMergeMap() lost existing key 'a'")
	}
	if result["b"] != "2" {
		t.Error("deepMergeMap() did not add new key 'b'")
	}
}

func TestDeepMergeMap_RecursivelyMergesNestedMaps(t *testing.T) {
	dst := map[string]interface{}{
		"outer": map[string]interface{}{"inner-a": "1"},
	}
	src := map[string]interface{}{
		"outer": map[string]interface{}{"inner-b": "2"},
	}
	result := deepMergeMap(dst, src)
	outer, ok := result["outer"].(map[string]interface{})
	if !ok {
		t.Fatal("outer is not a map")
	}
	if outer["inner-a"] != "1" {
		t.Error("lost inner-a in nested merge")
	}
	if outer["inner-b"] != "2" {
		t.Error("did not add inner-b in nested merge")
	}
}

func TestDeepMergeMap_MergesSlicesUnion(t *testing.T) {
	dst := map[string]interface{}{
		"list": []interface{}{"a", "b"},
	}
	src := map[string]interface{}{
		"list": []interface{}{"b", "c"},
	}
	result := deepMergeMap(dst, src)
	list, _ := result["list"].([]interface{})
	// Should contain a, b (once), c
	found := map[interface{}]int{}
	for _, v := range list {
		found[v]++
	}
	if found["a"] != 1 {
		t.Errorf("slice merge: 'a' count = %d, want 1", found["a"])
	}
	if found["b"] != 1 {
		t.Errorf("slice merge: 'b' count = %d, want 1 (no duplicates)", found["b"])
	}
	if found["c"] != 1 {
		t.Errorf("slice merge: 'c' count = %d, want 1", found["c"])
	}
}

func TestDeepMergeMap_ScalarSrcWins(t *testing.T) {
	dst := map[string]interface{}{"k": "old"}
	src := map[string]interface{}{"k": "new"}
	result := deepMergeMap(dst, src)
	if result["k"] != "new" {
		t.Errorf("deepMergeMap() scalar: got %q, want 'new'", result["k"])
	}
}

// ─── injectJSONFile ───────────────────────────────────────────────────────────

func TestInjectJSONFile_AddsNewKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	existing := `{"a": "1"}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	template := []byte(`{"b": "2"}`)
	if err := injectJSONFile(path, template); err != nil {
		t.Fatalf("injectJSONFile() error = %v", err)
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(got), `"a"`) {
		t.Error("injectJSONFile() lost existing key 'a'")
	}
	if !strings.Contains(string(got), `"b"`) {
		t.Error("injectJSONFile() did not add new key 'b'")
	}
}

func TestInjectJSONFile_PreservesExistingArrayAndAddsNewItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	existing := `{"ignorePaths": ["node_modules", "dist"]}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	template := []byte(`{"ignorePaths": ["dist", "coverage", "tmp"]}`)
	if err := injectJSONFile(path, template); err != nil {
		t.Fatalf("injectJSONFile() error = %v", err)
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	for _, want := range []string{"node_modules", "dist", "coverage", "tmp"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("injectJSONFile() result missing %q", want)
		}
	}
	// "dist" should not be duplicated.
	if strings.Count(string(got), `"dist"`) != 1 {
		t.Errorf("injectJSONFile() duplicated 'dist': %s", got)
	}
}

func TestStripJSONCComments_RemovesTrailingCommas(t *testing.T) {
	input := `{"list": ["a", "b",], "obj": {"k": "v",}}`
	got := string(stripJSONCComments([]byte(input)))
	if strings.Contains(got, ",]") {
		t.Errorf("stripJSONCComments() left trailing comma before ]: %q", got)
	}
	if strings.Contains(got, ",}") {
		t.Errorf("stripJSONCComments() left trailing comma before }: %q", got)
	}
	// Result must be valid JSON.
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Errorf("stripJSONCComments() result is not valid JSON: %v – output: %q", err, got)
	}
}

func TestInjectJSONFile_HandlesJSONCComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.jsonc")

	// JSONC file with comments and trailing commas (common in real .jsonc files).
	existing := `{
  "language": "en-US",
  // a comment
  "ignorePaths": [
    "**/.git/**", // always ignored
  ]
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	template := []byte(`{"ignorePaths": ["**/node_modules/**"]}`)
	if err := injectJSONFile(path, template); err != nil {
		t.Fatalf("injectJSONFile() error = %v", err)
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(got), "node_modules") {
		t.Error("injectJSONFile() did not add node_modules from template")
	}
	if !strings.Contains(string(got), ".git") {
		t.Error("injectJSONFile() lost existing .git entry")
	}
	if !strings.Contains(string(got), "en-US") {
		t.Error("injectJSONFile() lost existing language setting")
	}
}

// ─── injectYAMLFile ───────────────────────────────────────────────────────────

func TestInjectYAMLFile_AddsNewKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	existing := "language: en-US\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	template := []byte("version: \"0.2\"\n")
	if err := injectYAMLFile(path, template); err != nil {
		t.Fatalf("injectYAMLFile() error = %v", err)
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(got), "language") {
		t.Error("injectYAMLFile() lost existing key 'language'")
	}
	if !strings.Contains(string(got), "version") {
		t.Error("injectYAMLFile() did not add 'version' from template")
	}
}

func TestInjectYAMLFile_MergesNestedMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	existing := "features:\n  go: true\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	template := []byte("features:\n  python: true\n")
	if err := injectYAMLFile(path, template); err != nil {
		t.Fatalf("injectYAMLFile() error = %v", err)
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(got), "go") {
		t.Error("injectYAMLFile() lost existing 'go' feature")
	}
	if !strings.Contains(string(got), "python") {
		t.Error("injectYAMLFile() did not add 'python' from template")
	}
}

// ─── injectIntoExisting – fallback ───────────────────────────────────────────

// TestInjectFallsBackToCombineForUnknownExtension ensures that file types not
// handled by inject fall back to the combine (union) strategy gracefully.
func TestInjectFallsBackToCombineForUnknownExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := "key = \"value\"\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	// Template has an additional key – combine should produce both.
	template := []byte("extra = \"added\"\n")

	_, err := injectIntoExisting(path, template)
	if err != nil {
		t.Fatalf("injectIntoExisting() error = %v", err)
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(got), "key") {
		t.Error("fallback combine lost existing key")
	}
	if !strings.Contains(string(got), "extra") {
		t.Error("fallback combine did not add extra key from template")
	}
	if strings.Contains(string(got), "<<<<<<<") {
		t.Error("fallback combine produced conflict markers")
	}
}

// ─── applyBoilerplateFile with inject strategy ───────────────────────────────

func TestApplyBoilerplateFile_InjectCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.json")
	content := []byte(`{"key": "value"}`)

	conflicts, err := applyBoilerplateFile(path, content, "inject")
	if err != nil {
		t.Fatalf("applyBoilerplateFile(inject) error = %v", err)
	}
	if conflicts {
		t.Error("applyBoilerplateFile(inject) reported conflicts for a new file")
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(got), "key") {
		t.Error("new file missing expected content")
	}
}

func TestApplyBoilerplateFile_InjectNeverReportsConflicts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	existing := `{"a": "existing"}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	conflicts, err := applyBoilerplateFile(path, []byte(`{"b": "template"}`), "inject")
	if err != nil {
		t.Fatalf("applyBoilerplateFile(inject) error = %v", err)
	}
	if conflicts {
		t.Error("inject strategy should never report conflicts")
	}
}
