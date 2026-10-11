package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// ReadDocument parses one strict YAML mapping followed by untouched UTF-8
// Markdown or HTML. Application validators run before the document is returned.
func ReadDocument(path string, out any) (body string, err error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	body, e = ParseDocument(b, out)
	if e != nil {
		return "", fmt.Errorf("%s: %w", path, e)
	}
	return body, nil
}
func documentHeader(b []byte) (header []byte, body string, node *yaml.Node, err error) {
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return nil, "", nil, fmt.Errorf("document must be UTF-8 without NUL bytes")
	}
	if len(b) >= 3 && bytes.Equal(b[:3], []byte{0xef, 0xbb, 0xbf}) {
		b = b[3:]
	}
	first := bytes.IndexByte(b, '\n')
	if first < 0 || strings.TrimSuffix(string(b[:first]), "\r") != "---" {
		return nil, "", nil, fmt.Errorf("line 1: expected --- YAML header")
	}
	start := first + 1
	end := -1
	bodyStart := 0
	for pos := start; pos <= len(b); {
		next := bytes.IndexByte(b[pos:], '\n')
		lineEnd := len(b)
		if next >= 0 {
			lineEnd = pos + next
		}
		line := strings.TrimSuffix(string(b[pos:lineEnd]), "\r")
		if line == "---" {
			end = pos
			bodyStart = lineEnd
			if next >= 0 {
				bodyStart++
			}
			break
		}
		if next < 0 {
			break
		}
		pos = lineEnd + 1
	}
	if end < 0 {
		return nil, "", nil, fmt.Errorf("unterminated YAML header: expected closing ---")
	}
	header = b[start:end]
	node = &yaml.Node{}
	decoder := yaml.NewDecoder(bytes.NewReader(header))
	if e := decoder.Decode(node); e != nil {
		return nil, "", nil, e
	}
	var extra yaml.Node
	if e := decoder.Decode(&extra); e != io.EOF {
		return nil, "", nil, fmt.Errorf("header must contain exactly one YAML document")
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return nil, "", nil, fmt.Errorf("header must be a YAML mapping")
	}
	if e := checkNode(node.Content[0]); e != nil {
		return nil, "", nil, e
	}
	schemaFound := false
	for i := 0; i < len(node.Content[0].Content); i += 2 {
		if node.Content[0].Content[i].Value == "schema" {
			n := node.Content[0].Content[i+1]
			if n.Tag != "!!int" || n.Value != "1" {
				return nil, "", nil, fmt.Errorf("line %d: schema must be integer 1", n.Line+1)
			}
			schemaFound = true
		}
	}
	if !schemaFound {
		return nil, "", nil, fmt.Errorf("schema: 1 is required")
	}
	return header, string(b[bodyStart:]), node, nil
}

// DocumentKind reads the YAML discriminator without guessing from body text.
// Call ReadDocument with the corresponding type for complete field validation.
func DocumentKind(b []byte) (string, error) {
	_, _, node, e := documentHeader(b)
	if e != nil {
		return "", e
	}
	for i := 0; i < len(node.Content[0].Content); i += 2 {
		if node.Content[0].Content[i].Value == "kind" {
			n := node.Content[0].Content[i+1]
			if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || strings.TrimSpace(n.Value) == "" {
				return "", fmt.Errorf("line %d: kind must be a nonempty string", n.Line+1)
			}
			return n.Value, nil
		}
	}
	return "", fmt.Errorf("kind is required")
}

func ParseDocument(b []byte, out any) (string, error) {
	header, body, node, e := documentHeader(b)
	if e != nil {
		return "", e
	}
	if out == nil || reflect.ValueOf(out).Kind() != reflect.Pointer || reflect.ValueOf(out).IsNil() {
		return "", fmt.Errorf("document target must be a nonnil pointer")
	}
	// Decode into a fresh value, so a failed parse cannot leave a half-updated
	// object or preserve fields missing from the new document.
	fresh := reflect.New(reflect.ValueOf(out).Elem().Type())
	strict := yaml.NewDecoder(bytes.NewReader(header))
	strict.KnownFields(true)
	if e := strict.Decode(fresh.Interface()); e != nil {
		var fields *yaml.TypeError
		if errors.As(e, &fields) {
			for _, detail := range fields.Errors {
				if strings.Contains(detail, "field ") && strings.Contains(detail, "not found in type") {
					hint := "Check accepted fields with engine_reference (formats and examples)."
					for i := 0; i < len(node.Content[0].Content); i += 2 {
						if node.Content[0].Content[i].Value == "kind" && node.Content[0].Content[i+1].Value == "results" {
							hint = "For results, use interpretation.mapping and students[].grade.components; attempt accepts only attempt and submitted_at. See engine_reference formats and examples."
							break
						}
					}
					return "", fmt.Errorf("%w; %s", e, hint)
				}
			}
		}
		return "", e
	}
	// After the closing delimiter every byte is author/student content. A
	// Markdown horizontal rule or text resembling YAML is still body data.
	if v, ok := fresh.Interface().(interface{ ValidateBody(string) error }); ok {
		if e := v.ValidateBody(body); e != nil {
			return "", e
		}
	}
	if v, ok := fresh.Interface().(interface{ Validate() error }); ok {
		if e := v.Validate(); e != nil {
			return "", e
		}
	}
	reflect.ValueOf(out).Elem().Set(fresh.Elem())
	return body, nil
}
func checkNode(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return fmt.Errorf("line %d: YAML anchors/aliases are not supported", n.Line+1)
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				return fmt.Errorf("line %d: mapping keys must be strings", k.Line+1)
			}
			if k.Value == "<<" || seen[k.Value] {
				return fmt.Errorf("line %d: duplicate or merge key %q", k.Line+1, k.Value)
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if e := checkNode(c); e != nil {
			return e
		}
	}
	return nil
}
func WriteDocument(path string, header any, body string) error {
	if header == nil || (reflect.ValueOf(header).Kind() == reflect.Pointer && reflect.ValueOf(header).IsNil()) {
		return fmt.Errorf("nil header")
	}
	validatedHeader := header
	if value := reflect.ValueOf(header); value.Kind() != reflect.Pointer {
		copy := reflect.New(value.Type())
		copy.Elem().Set(value)
		validatedHeader = copy.Interface()
	}
	// Validate before marshalling, which would otherwise turn an unset Decimal
	// into an explicit zero and lose the missing-field validation.
	if v, ok := validatedHeader.(interface{ ValidateBody(string) error }); ok {
		if e := v.ValidateBody(body); e != nil {
			return fmt.Errorf("cannot write invalid document: %w", e)
		}
	}
	if v, ok := validatedHeader.(interface{ Validate() error }); ok {
		if e := v.Validate(); e != nil {
			return fmt.Errorf("cannot write invalid document: %w", e)
		}
	}
	h, e := yaml.Marshal(header)
	if e != nil {
		return e
	}
	b := append([]byte("---\n"), h...)
	b = append(b, []byte("---\n")...)
	b = append(b, []byte(body)...)
	// The same strict path validates output before replacing an existing file.
	t := reflect.TypeOf(header)
	if t == nil {
		return fmt.Errorf("nil header")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	fresh := reflect.New(t).Interface()
	if _, e = ParseDocument(b, fresh); e != nil {
		return fmt.Errorf("cannot write invalid document: %w", e)
	}
	return WriteFile(path, b, 0644)
}

// WriteFile writes a sibling temporary file, closes it, then replaces the
// destination. No delete-before-rename window can destroy the previous file.
func WriteFile(path string, b []byte, mode os.FileMode) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".cse-write-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	// Windows readers/scanners can briefly deny replacement while holding the
	// destination open. Rename failure leaves the old document intact, so a
	// short bounded retry of this local replacement is safe.
	for attempt := 0; ; attempt++ {
		e = os.Rename(tmp, path)
		if e == nil || runtime.GOOS != "windows" || attempt >= 4 || (!errors.Is(e, os.ErrPermission) && !errors.Is(e, syscall.Errno(32))) {
			break
		}
		time.Sleep(time.Duration(10<<attempt) * time.Millisecond)
	}
	if e != nil {
		return fmt.Errorf("replace %s: %w", path, e)
	}
	return nil
}
