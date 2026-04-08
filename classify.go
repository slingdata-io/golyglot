package golyglot

import (
	"encoding/json"
	"fmt"
	"strings"
)

// StatementType classifies a SQL statement.
type StatementType int

const (
	StmtQuery StatementType = iota // SELECT, WITH, UNION, VALUES, etc.
	StmtDDL                        // CREATE, ALTER, DROP, TRUNCATE, RENAME
	StmtDML                        // INSERT, UPDATE, DELETE, MERGE
	StmtOther                      // SET, GRANT, EXPLAIN, ANALYZE, etc.
)

func (t StatementType) String() string {
	switch t {
	case StmtQuery:
		return "query"
	case StmtDDL:
		return "ddl"
	case StmtDML:
		return "dml"
	case StmtOther:
		return "other"
	default:
		return "unknown"
	}
}

// ClassifiedStatement holds a parsed SQL statement with its type and original SQL.
type ClassifiedStatement struct {
	Type    StatementType
	TypeKey string // the top-level AST key, e.g. "select", "create_table"
	SQL     string // regenerated SQL for this statement
	Node    json.RawMessage
}

// queryKeys are AST node types that produce result sets (the "model" in a model file).
var queryKeys = map[string]bool{
	"select":        true,
	"set_operation": true,
	"union":         true,
	"intersect":     true,
	"except":        true,
	"values":        true,
}

// ddlKeys are AST node types that define or modify schema objects.
var ddlKeys = map[string]bool{
	"create_table":    true,
	"create_view":     true,
	"create_index":    true,
	"create_schema":   true,
	"create_function": true,
	"create_sequence": true,
	"create_type":     true,
	"alter_table":     true,
	"drop_table":      true,
	"drop_view":       true,
	"drop_index":      true,
	"drop_schema":     true,
	"drop_function":   true,
	"drop_sequence":   true,
	"drop_type":       true,
	"truncate":        true,
	"rename_table":    true,
	"comment_on":      true,
}

// dmlKeys are AST node types that modify data.
var dmlKeys = map[string]bool{
	"insert": true,
	"update": true,
	"delete": true,
	"merge":  true,
	"copy":   true,
}

// classifyNode determines the statement type from the top-level AST key.
func classifyNode(key string) StatementType {
	k := strings.ToLower(key)
	if queryKeys[k] {
		return StmtQuery
	}
	if ddlKeys[k] {
		return StmtDDL
	}
	if dmlKeys[k] {
		return StmtDML
	}
	return StmtOther
}

// ClassifyStatements parses SQL and classifies each statement.
// Uses polyglot to parse and generate SQL back for each statement.
func ClassifyStatements(sql, dialect string) ([]ClassifiedStatement, error) {
	result, err := Parse(sql, dialect)
	if err != nil {
		return nil, err
	}

	// Parse the JSON array of AST nodes
	var nodes []json.RawMessage
	if err := json.Unmarshal(result.AST, &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse AST JSON: %w", err)
	}

	var stmts []ClassifiedStatement
	for _, node := range nodes {
		// Each node is a JSON object with a single top-level key (the statement type)
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(node, &obj); err != nil {
			return nil, fmt.Errorf("failed to parse AST node: %w", err)
		}

		// Get the single top-level key
		var typeKey string
		for k := range obj {
			typeKey = k
			break
		}

		// Generate SQL for this individual statement
		singleAST := "[" + string(node) + "]"
		genResult, err := Generate(singleAST, dialect)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SQL for %s statement: %w", typeKey, err)
		}

		// The generate result is a JSON object with a "sql" field or array
		generatedSQL := genResult.SQL

		stmts = append(stmts, ClassifiedStatement{
			Type:    classifyNode(typeKey),
			TypeKey: typeKey,
			SQL:     generatedSQL,
			Node:    node,
		})
	}

	return stmts, nil
}
