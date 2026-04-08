package golyglot

import (
	"testing"
)

func TestClassifyStatements(t *testing.T) {
	sql := `CREATE TEMP TABLE tmp AS SELECT * FROM orders WHERE created_at > '2024-01-01';
SELECT id, name FROM tmp JOIN customers USING (id);
DROP TABLE tmp;`

	stmts, err := ClassifyStatements(sql, "postgres")
	if err != nil {
		t.Fatalf("ClassifyStatements failed: %v", err)
	}

	if len(stmts) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(stmts))
	}

	if stmts[0].Type != StmtDDL {
		t.Errorf("stmt 0: expected DDL, got %s (key: %s)", stmts[0].Type, stmts[0].TypeKey)
	}
	if stmts[1].Type != StmtQuery {
		t.Errorf("stmt 1: expected Query, got %s (key: %s)", stmts[1].Type, stmts[1].TypeKey)
	}
	if stmts[2].Type != StmtDDL {
		t.Errorf("stmt 2: expected DDL, got %s (key: %s)", stmts[2].Type, stmts[2].TypeKey)
	}

	for i, stmt := range stmts {
		t.Logf("stmt %d: type=%s key=%s sql=%s", i, stmt.Type, stmt.TypeKey, stmt.SQL)
	}
}

func TestClassifyDML(t *testing.T) {
	sql := `INSERT INTO t VALUES (1); UPDATE t SET x = 1; DELETE FROM t WHERE id = 1;`

	stmts, err := ClassifyStatements(sql, "postgres")
	if err != nil {
		t.Fatalf("ClassifyStatements failed: %v", err)
	}

	for i, stmt := range stmts {
		if stmt.Type != StmtDML {
			t.Errorf("stmt %d: expected DML, got %s (key: %s)", i, stmt.Type, stmt.TypeKey)
		}
	}
}
