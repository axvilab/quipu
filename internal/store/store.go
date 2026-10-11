// Package store keeps collected transcript data as Parquet files and queries them with DuckDB.
//
// Each transcript becomes one Parquet file, rewritten whole when the transcript grows, so the
// data survives Claude Code's 30-day cleanup and collection can be repeated safely.
// All files are exposed to SQL as one view, "responses".
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2" // registers the "duckdb" database/sql driver

	"github.com/axvilab/quipu/internal/claudecode"
)

// Source identifies the transcript a set of responses came from.
type Source struct {
	Path       string // transcript file; only used to name the Parquet file
	Profile    string
	ProjectDir string
	SessionID  string
	Kind       claudecode.TranscriptKind
}

// columns is the schema of the responses view, in insert order.
var columns = []struct{ name, typ string }{
	{"profile", "VARCHAR"},
	{"project_dir", "VARCHAR"},
	{"session_id", "VARCHAR"},
	{"transcript_kind", "VARCHAR"},
	{"message_id", "VARCHAR"},
	{"request_id", "VARCHAR"},
	{"ts", "TIMESTAMP"},
	{"version", "VARCHAR"},
	{"cwd", "VARCHAR"},
	{"git_branch", "VARCHAR"},
	{"model", "VARCHAR"},
	{"stop_reason", "VARCHAR"},
	{"sidechain", "BOOLEAN"},
	{"input_tokens", "BIGINT"},
	{"cache_read_tokens", "BIGINT"},
	{"cache_creation_tokens", "BIGINT"},
	{"cache_creation_5m_tokens", "BIGINT"},
	{"cache_creation_1h_tokens", "BIGINT"},
	{"output_tokens", "BIGINT"},
	{"cache_miss_type", "VARCHAR"},
	{"cache_miss_tokens", "BIGINT"},
}

// Store is a directory of Parquet files plus an in-memory DuckDB to query them.
type Store struct {
	dir string // <data dir>/responses
	db  *sql.DB
}

// Open creates the store under dataDir if needed. The directory is private to the user.
func Open(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "responses")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	return &Store{dir: dir, db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// WriteResponses replaces the stored data of one transcript with rs.
// The file is written to a temporary name and renamed, so readers never see a partial file.
func (s *Store) WriteResponses(ctx context.Context, src Source, rs []claudecode.Response) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "CREATE OR REPLACE TEMP TABLE batch ("+columnDefs()+")"); err != nil {
		return fmt.Errorf("create batch table: %w", err)
	}
	defer conn.ExecContext(context.Background(), "DROP TABLE IF EXISTS batch")

	if err := insertRows(ctx, conn, src, rs); err != nil {
		return err
	}

	final := filepath.Join(s.dir, fileName(src.Path))
	tmp := final + ".tmp"
	copySQL := fmt.Sprintf("COPY batch TO %s (FORMAT parquet, COMPRESSION zstd)", quote(tmp))
	if _, err := conn.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("write parquet: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func insertRows(ctx context.Context, conn *sql.Conn, src Source, rs []claudecode.Response) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ")
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO batch VALUES ("+placeholders+")")
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, r := range rs {
		var missType, missTokens any
		if r.CacheMiss != nil {
			missType, missTokens = r.CacheMiss.Type, r.CacheMiss.MissedTokens
		}
		u := r.Usage
		_, err := stmt.ExecContext(ctx,
			src.Profile, src.ProjectDir, src.SessionID, string(src.Kind),
			r.MessageID, r.RequestID, r.Timestamp.UTC(), r.Version, r.Cwd, r.GitBranch,
			r.Model, r.StopReason, r.Sidechain,
			u.InputTokens, u.CacheReadTokens, u.CacheCreationTokens,
			u.CacheCreation.Ephemeral5m, u.CacheCreation.Ephemeral1h, u.OutputTokens,
			missType, missTokens,
		)
		if err != nil {
			return fmt.Errorf("insert response %s: %w", r.MessageID, err)
		}
	}
	return tx.Commit()
}

// Query runs SQL against the "responses" view over all stored files.
func (s *Store) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	// The view is rebuilt on every query, so it always covers the files on disk right now.
	// The in-memory database is shared by all connections of s.db, so a plain view is visible
	// to the query below whichever pooled connection runs it.
	if _, err := s.db.ExecContext(ctx, "CREATE OR REPLACE VIEW responses AS "+s.source()); err != nil {
		return nil, fmt.Errorf("create responses view: %w", err)
	}
	return s.db.QueryContext(ctx, query, args...)
}

// source is the SELECT behind the responses view. With no files yet it is an empty relation of
// the same shape, so queries work on a fresh store.
func (s *Store) source() string {
	files, _ := filepath.Glob(filepath.Join(s.dir, "*.parquet"))
	if len(files) == 0 {
		var cols []string
		for _, c := range columns {
			cols = append(cols, fmt.Sprintf("NULL::%s AS %s", c.typ, c.name))
		}
		return "SELECT " + strings.Join(cols, ", ") + " WHERE false"
	}
	return fmt.Sprintf("SELECT * FROM read_parquet(%s, union_by_name = true)",
		quote(filepath.Join(s.dir, "*.parquet")))
}

func columnDefs() string {
	defs := make([]string, len(columns))
	for i, c := range columns {
		defs[i] = c.name + " " + c.typ
	}
	return strings.Join(defs, ", ")
}

// fileName derives a stable Parquet file name from the transcript path.
func fileName(transcriptPath string) string {
	sum := sha256.Sum256([]byte(transcriptPath))
	return hex.EncodeToString(sum[:12]) + ".parquet"
}

// quote makes a SQL string literal.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
