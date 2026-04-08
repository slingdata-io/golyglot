package golyglot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func TestLibVersion(t *testing.T) {
	v, err := LibVersion()
	if err != nil {
		t.Fatalf("LibVersion failed: %v", err)
	}
	if v == "" {
		t.Fatal("LibVersion returned empty string")
	}
	if !strings.HasPrefix(v, "0.") {
		t.Fatalf("unexpected version: %s", v)
	}
	t.Logf("polyglot-sql version: %s", v)
}

func TestDialectCount(t *testing.T) {
	count, err := DialectCount()
	if err != nil {
		t.Fatalf("DialectCount failed: %v", err)
	}
	if count < 20 {
		t.Fatalf("expected at least 20 dialects, got %d", count)
	}
	t.Logf("dialect count: %d", count)
}

func TestDialectList(t *testing.T) {
	list, err := DialectList()
	if err != nil {
		t.Fatalf("DialectList failed: %v", err)
	}
	if !strings.Contains(list, "postgres") {
		t.Fatalf("dialect list missing 'postgres': %s", list)
	}
	t.Logf("dialects: %s", list)
}

func TestParseSingleSelect(t *testing.T) {
	result, err := Parse("SELECT 1", "generic")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if result.AST == nil {
		t.Fatal("AST is nil")
	}

	var nodes []map[string]json.RawMessage
	if err := json.Unmarshal(result.AST, &nodes); err != nil {
		t.Fatalf("failed to unmarshal AST: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if _, ok := nodes[0]["select"]; !ok {
		t.Fatalf("expected 'select' key, got: %v", nodeKeys(nodes[0]))
	}
}

func TestParseMultiStatement(t *testing.T) {
	sql := `CREATE TABLE tmp (id INT); SELECT * FROM tmp; DROP TABLE tmp;`
	result, err := Parse(sql, "postgres")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var nodes []map[string]json.RawMessage
	if err := json.Unmarshal(result.AST, &nodes); err != nil {
		t.Fatalf("failed to unmarshal AST: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	expectedKeys := []string{"create_table", "select", "drop_table"}
	for i, expected := range expectedKeys {
		if _, ok := nodes[i][expected]; !ok {
			t.Errorf("node %d: expected key '%s', got: %v", i, expected, nodeKeys(nodes[i]))
		}
	}
}

func TestParsePostgresDialect(t *testing.T) {
	sql := `SELECT * FROM "MyTable" WHERE name = 'test'`
	result, err := Parse(sql, "postgres")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if result.AST == nil {
		t.Fatal("AST is nil")
	}
}

func TestParseCTE(t *testing.T) {
	sql := `WITH cte AS (SELECT 1 AS x) SELECT * FROM cte`
	result, err := Parse(sql, "generic")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var nodes []map[string]json.RawMessage
	if err := json.Unmarshal(result.AST, &nodes); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("CTE should be 1 statement, got %d", len(nodes))
	}
	// CTE wraps a select, should have "select" key (with a with clause inside)
	keys := nodeKeys(nodes[0])
	t.Logf("CTE node keys: %v", keys)
}

func TestParseUnion(t *testing.T) {
	sql := `SELECT 1 UNION ALL SELECT 2`
	result, err := Parse(sql, "generic")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var nodes []map[string]json.RawMessage
	if err := json.Unmarshal(result.AST, &nodes); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("UNION should be 1 statement, got %d", len(nodes))
	}
	keys := nodeKeys(nodes[0])
	t.Logf("UNION node keys: %v", keys)
}

func TestFormat(t *testing.T) {
	result, err := Format("select a,b,c from t where x=1", "postgres")
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	if result.SQL == "" {
		t.Fatal("Format returned empty SQL")
	}
	t.Logf("formatted: %s", result.SQL)
}

func TestTranspile(t *testing.T) {
	result, err := Transpile("SELECT IFNULL(a, b) FROM t", "mysql", "postgres")
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}
	if result.SQL == "" {
		t.Fatal("Transpile returned empty SQL")
	}
	t.Logf("transpiled: %s", result.SQL)
}

func TestValidate(t *testing.T) {
	result, err := Validate("SELECT 1", "postgres")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid SQL, got errors: %s", result.Errors)
	}
}

func TestValidateInvalid(t *testing.T) {
	result, err := Validate("SELEC 1", "postgres")
	if err != nil {
		// parse error is acceptable
		t.Logf("validation returned error (expected): %v", err)
		return
	}
	t.Logf("valid=%v, errors=%s", result.Valid, result.Errors)
}

func TestTokenize(t *testing.T) {
	result, err := Tokenize("SELECT 1", "generic")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	if result.Tokens == nil {
		t.Fatal("Tokens is nil")
	}
	t.Logf("tokens: %s", string(result.Tokens)[:min(200, len(result.Tokens))])
}

func TestGenerate(t *testing.T) {
	// First parse to get AST
	parsed, err := Parse("SELECT 1 AS x", "generic")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Then generate back
	result, err := Generate(string(parsed.AST), "generic")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	t.Logf("generated: %s", result.SQL)
}

// Helper to get keys from a map
func nodeKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
