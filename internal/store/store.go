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
	"sync"
	"time"

	_ "github.com/duckdb/duckdb-go/v2" // registers the "duckdb" database/sql driver

	"github.com/axvilab/quipu/internal/claudecode"
)

// Source identifies the transcript a set of responses came from.
type Source struct {
	Path       string    // transcript file; only used to name the Parquet file
	ModTime    time.Time // transcript modification time when it was read
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
	{"plugins", "VARCHAR"}, // comma-separated; added in schema 2
	{"skills", "VARCHAR"},  // comma-separated; added in schema 3
}

// schema is the version of the file layout above. Files of older versions are rewritten when
// their transcript still exists, and read as they are (missing columns are NULL) when it is gone.
const schema = 3

// Store is a directory of Parquet files plus an in-memory DuckDB to query them.
type Store struct {
	dir string // <data dir>/responses
	db  *sql.DB
	// view serializes rebuilding the responses view: DuckDB fails concurrent CREATE OR REPLACE
	// of the same view with a write-write conflict, which the UI's parallel requests hit.
	view sync.Mutex
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

// UpToDate reports whether the transcript at path has not changed since it was stored with the
// current schema, so collecting it again would change nothing.
func (s *Store) UpToDate(path string, modTime time.Time) bool {
	fi, err := os.Stat(filepath.Join(s.dir, fileName(path)))
	return err == nil && !fi.ModTime().Before(modTime)
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
	// A unique temporary name: the daemon and the UI may collect the same transcript at once.
	tmp := fmt.Sprintf("%s.%d.tmp", final, time.Now().UnixNano())
	copySQL := fmt.Sprintf("COPY batch TO %s (FORMAT parquet, COMPRESSION zstd)", quote(tmp))
	if _, err := conn.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("write parquet: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	// The file carries the transcript's modification time from before it was read. If lines are
	// appended while we read, the transcript becomes newer than this and is collected again.
	if !src.ModTime.IsZero() {
		if err := os.Chtimes(tmp, src.ModTime, src.ModTime); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	// The same transcript under an older schema would be counted twice.
	for _, old := range olderFileNames(src.Path) {
		os.Remove(filepath.Join(s.dir, old))
	}
	return nil
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
			missType, missTokens, strings.Join(r.Plugins, ","), strings.Join(r.Skills, ","),
		)
		if err != nil {
			return fmt.Errorf("insert response %s: %w", r.MessageID, err)
		}
	}
	return tx.Commit()
}

// Empty reports whether nothing has been stored yet.
func (s *Store) Empty() bool {
	files, _ := filepath.Glob(filepath.Join(s.dir, "*.parquet"))
	return len(files) == 0
}

// Query runs SQL against the "responses" view over all stored files.
func (s *Store) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	// The view is rebuilt on every query, so it always covers the files on disk right now.
	// The in-memory database is shared by all connections of s.db, so a plain view is visible
	// to the query below whichever pooled connection runs it.
	// The query is bound to the view when it starts, so the lock is held only until then.
	s.view.Lock()
	defer s.view.Unlock()
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
		return emptyRelation()
	}
	// The empty relation of the full shape makes every column exist even when all files are of
	// an older schema.
	return fmt.Sprintf("SELECT * FROM read_parquet(%s, union_by_name = true) UNION ALL BY NAME (%s)",
		quote(filepath.Join(s.dir, "*.parquet")), emptyRelation())
}

func emptyRelation() string {
	var cols []string
	for _, c := range columns {
		cols = append(cols, fmt.Sprintf("NULL::%s AS %s", c.typ, c.name))
	}
	return "SELECT " + strings.Join(cols, ", ") + " WHERE false"
}

func columnDefs() string {
	defs := make([]string, len(columns))
	for i, c := range columns {
		defs[i] = c.name + " " + c.typ
	}
	return strings.Join(defs, ", ")
}

// fileName derives a stable Parquet file name from the transcript path and the schema version.
func fileName(transcriptPath string) string {
	sum := sha256.Sum256([]byte(transcriptPath))
	return fmt.Sprintf("%s.v%d.parquet", hex.EncodeToString(sum[:12]), schema)
}

// olderFileNames are the names a transcript's file had under earlier schemas: no version suffix
// under schema 1, .v<n> after it.
func olderFileNames(transcriptPath string) []string {
	sum := sha256.Sum256([]byte(transcriptPath))
	base := hex.EncodeToString(sum[:12])
	names := []string{base + ".parquet"}
	for v := 2; v < schema; v++ {
		names = append(names, fmt.Sprintf("%s.v%d.parquet", base, v))
	}
	return names
}

// quote makes a SQL string literal.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
