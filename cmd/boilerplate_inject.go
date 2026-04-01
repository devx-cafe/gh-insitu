package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// injectIntoExisting applies templateContent into an existing file on disk
// using a programmatic, format-aware merge. The injection strategy adds or
// updates keys/items from the template without producing conflict markers.
//
// Supported formats (detected by file extension):
//   - .json, .jsonc → JSON deep-merge; JSONC comments are stripped on write
//   - .yml, .yaml   → YAML deep-merge
//
// All other file types fall back to the line-based mergeIntoExisting strategy.
//
// The function always returns hadConflicts=false because injection is
// conflict-free by design.
func injectIntoExisting(path string, templateContent []byte) (hadConflicts bool, err error) {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	switch {
	case ext == ".json" || ext == ".jsonc":
		return false, injectJSONFile(path, templateContent)
	case ext == ".yml" || ext == ".yaml":
		return false, injectYAMLFile(path, templateContent)
	default:
		// TOML, INI, .gitconfig and all other types: fall back to git merge.
		_ = base
		return mergeIntoExisting(path, templateContent)
	}
}

// injectJSONFile deep-merges the template JSON into the existing JSON file at
// path. JSONC comments (// and /* */) are stripped from both inputs before
// parsing; the output is always standard JSON with 2-space indentation.
func injectJSONFile(path string, templateContent []byte) error {
	existing, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	existingMap, err := parseJSON(existing)
	if err != nil {
		return fmt.Errorf("failed to parse existing JSON in %s: %w", path, err)
	}
	templateMap, err := parseJSON(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template JSON for %s: %w", path, err)
	}

	merged := deepMergeMap(existingMap, templateMap)

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged JSON for %s: %w", path, err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("failed to write merged JSON to %s: %w", path, err)
	}
	return nil
}

// injectYAMLFile deep-merges the template YAML into the existing YAML file at
// path. The output preserves the existing file's structure and appends/updates
// keys from the template. The document is re-serialized with 2-space indent.
func injectYAMLFile(path string, templateContent []byte) error {
	existing, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	existingMap, err := parseYAML(existing)
	if err != nil {
		return fmt.Errorf("failed to parse existing YAML in %s: %w", path, err)
	}
	templateMap, err := parseYAML(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template YAML for %s: %w", path, err)
	}

	merged := deepMergeMap(existingMap, templateMap)

	out, err := yaml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("failed to marshal merged YAML for %s: %w", path, err)
	}

	if err := os.WriteFile(path, out, 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("failed to write merged YAML to %s: %w", path, err)
	}
	return nil
}

// parseJSON strips JSONC-style comments and then unmarshals the data into a
// map[string]interface{}.
func parseJSON(data []byte) (map[string]interface{}, error) {
	clean := stripJSONCComments(data)
	var m map[string]interface{}
	if err := json.Unmarshal(clean, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// parseYAML unmarshals YAML data into a map[string]interface{}.
func parseYAML(data []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// deepMergeMap merges src into dst recursively and returns the merged map.
// The dst map is modified in place; a reference to it is also returned for
// convenience. Merge rules:
//
//   - Both values are maps      → recurse
//   - Both values are slices    → union (src items not already in dst are appended)
//   - src key is absent in dst  → add src value
//   - src key exists in dst and types differ (or scalar) → src value wins
func deepMergeMap(dst, src map[string]interface{}) map[string]interface{} {
	for k, srcVal := range src {
		dstVal, exists := dst[k]
		if !exists {
			dst[k] = srcVal
			continue
		}

		switch srcTyped := srcVal.(type) {
		case map[string]interface{}:
			if dstMap, ok := dstVal.(map[string]interface{}); ok {
				dst[k] = deepMergeMap(dstMap, srcTyped)
				continue
			}
		case []interface{}:
			if dstSlice, ok := dstVal.([]interface{}); ok {
				dst[k] = mergeSliceValues(dstSlice, srcTyped)
				continue
			}
		}
		// Scalar or mismatched types: src wins.
		dst[k] = srcVal
	}
	return dst
}

// mergeSliceValues returns a slice containing all elements of dst followed by
// any elements of src that are not already present in dst. Equality is
// determined by JSON-marshalled representation.
func mergeSliceValues(dst, src []interface{}) []interface{} {
	seen := make(map[string]bool, len(dst))
	for _, v := range dst {
		seen[jsonKey(v)] = true
	}

	result := make([]interface{}, len(dst))
	copy(result, dst)
	for _, v := range src {
		if k := jsonKey(v); !seen[k] {
			result = append(result, v)
			seen[k] = true
		}
	}
	return result
}

// jsonKey returns a stable string key for any JSON-serialisable value,
// used to test slice membership.
func jsonKey(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// stripJSONCComments removes JavaScript-style line comments (// …) and block
// comments (/* … */) from JSON/JSONC input, respecting string literals so
// that // or /* inside a quoted value are not stripped. It also removes
// trailing commas before ] and } which are valid in JSONC but not in JSON.
func stripJSONCComments(data []byte) []byte {
	var buf bytes.Buffer
	i := 0
	n := len(data)
	inString := false

	for i < n {
		c := data[i]

		if inString {
			buf.WriteByte(c)
			if c == '\\' && i+1 < n {
				// Escaped character – emit it verbatim and advance past it.
				i++
				buf.WriteByte(data[i])
			} else if c == '"' {
				inString = false
			}
			i++
			continue
		}

		if c == '"' {
			inString = true
			buf.WriteByte(c)
			i++
			continue
		}

		// Line comment: skip to end-of-line.
		if c == '/' && i+1 < n && data[i+1] == '/' {
			i += 2
			for i < n && data[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment: skip to closing */.
		if c == '/' && i+1 < n && data[i+1] == '*' {
			i += 2
			for i+1 < n && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2 // consume the closing */
			}
			continue
		}

		buf.WriteByte(c)
		i++
	}

	// Second pass: remove trailing commas before ] or } (invalid in JSON).
	return removeTrailingCommas(buf.Bytes())
}

// removeTrailingCommas strips commas that appear immediately before a closing
// ] or } (possibly with only whitespace in between). This converts JSONC
// trailing-comma syntax to valid JSON.
func removeTrailingCommas(data []byte) []byte {
	var buf bytes.Buffer
	i := 0
	n := len(data)

	for i < n {
		c := data[i]
		if c == ',' {
			// Look ahead past whitespace for ] or }.
			j := i + 1
			for j < n && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
				j++
			}
			if j < n && (data[j] == ']' || data[j] == '}') {
				// Skip the comma; the whitespace and closing bracket are emitted normally.
				i++
				continue
			}
		}
		buf.WriteByte(c)
		i++
	}
	return buf.Bytes()
}
