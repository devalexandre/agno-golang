package sandbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/devalexandre/agno-golang/agno/internal/sqldialect"
)

// ErrCheckpointNotFound is returned when a referenced object is missing.
var ErrCheckpointNotFound = errors.New("sandbox: checkpoint object not found")

// ErrCheckpointContent is returned when stored bytes do not match their reference.
var ErrCheckpointContent = errors.New("sandbox: checkpoint content does not match its reference")

// CheckpointRef addresses immutable checkpoint bytes by digest.
type CheckpointRef struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Validate checks the reference shape.
func (r CheckpointRef) Validate() error {
	if len(r.SHA256) != 64 {
		return errors.New("invalid checkpoint digest")
	}
	if _, err := hex.DecodeString(r.SHA256); err != nil {
		return errors.New("invalid checkpoint digest")
	}
	if r.Size < 1 {
		return errors.New("checkpoint size must be positive")
	}
	return nil
}

// CheckpointStore stores content-addressed checkpoint archives.
type CheckpointStore interface {
	// Put stores the bytes from r and returns their reference.
	Put(ctx context.Context, r io.Reader) (CheckpointRef, error)
	// Open returns the bytes; callers should treat a digest mismatch as fatal.
	Open(ctx context.Context, ref CheckpointRef) (io.ReadCloser, error)
	Delete(ctx context.Context, ref CheckpointRef) error
}

// hashTo copies r to w (bounded by max) returning its reference.
func hashTo(r io.Reader, w io.Writer, max int64) (CheckpointRef, error) {
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(h, w), io.LimitReader(r, max+1))
	if err != nil {
		return CheckpointRef{}, err
	}
	if n > max {
		return CheckpointRef{}, errors.New("checkpoint object exceeds its limit")
	}
	ref := CheckpointRef{SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}
	return ref, ref.Validate()
}

// verifyBytes checks b against ref.
func verifyBytes(b []byte, ref CheckpointRef) error {
	sum := sha256.Sum256(b)
	if int64(len(b)) != ref.Size || hex.EncodeToString(sum[:]) != ref.SHA256 {
		return ErrCheckpointContent
	}
	return nil
}

// FileCheckpointStore keeps objects as read-only files named by digest.
type FileCheckpointStore struct {
	Root     string
	MaxBytes int64 // default 1 GiB
}

// NewFileCheckpointStore creates root (0700) if needed.
func NewFileCheckpointStore(root string) (*FileCheckpointStore, error) {
	if fi, err := os.Lstat(root); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("checkpoint store root cannot be a symlink")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &FileCheckpointStore{Root: root, MaxBytes: 1 << 30}, nil
}

func (f *FileCheckpointStore) Put(_ context.Context, r io.Reader) (CheckpointRef, error) {
	tmp, err := os.CreateTemp(f.Root, ".upload-")
	if err != nil {
		return CheckpointRef{}, err
	}
	defer os.Remove(tmp.Name())
	ref, err := hashTo(r, tmp, f.MaxBytes)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return CheckpointRef{}, err
	}
	if err := os.Chmod(tmp.Name(), 0o400); err != nil {
		return CheckpointRef{}, err
	}
	target := filepath.Join(f.Root, ref.SHA256)
	if _, err := os.Stat(target); err == nil {
		return ref, nil // content-addressed: already stored
	}
	return ref, os.Rename(tmp.Name(), target)
}

func (f *FileCheckpointStore) Open(_ context.Context, ref CheckpointRef) (io.ReadCloser, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(f.Root, ref.SHA256))
	if os.IsNotExist(err) {
		return nil, ErrCheckpointNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := verifyBytes(b, ref); err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (f *FileCheckpointStore) Delete(_ context.Context, ref CheckpointRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(f.Root, ref.SHA256))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SQLCheckpointStore keeps objects in a Postgres (BYTEA) or SQLite (BLOB) table.
type SQLCheckpointStore struct {
	db       *sql.DB
	dialect  sqldialect.Dialect
	table    string
	MaxBytes int64 // default 256 MiB
}

// NewSQLCheckpointStore creates a store over table (default "agno_sandbox_checkpoints"); call Migrate.
func NewSQLCheckpointStore(db *sql.DB, dialect sqldialect.Dialect, table string) (*SQLCheckpointStore, error) {
	if err := dialect.Validate(); err != nil {
		return nil, err
	}
	if table == "" {
		table = "agno_sandbox_checkpoints"
	}
	if !sqldialect.ValidIdent(table) {
		return nil, fmt.Errorf("sandbox: invalid table name %q", table)
	}
	return &SQLCheckpointStore{db: db, dialect: dialect, table: table, MaxBytes: 256 << 20}, nil
}

// Migrate creates the table.
func (s *SQLCheckpointStore) Migrate(ctx context.Context) error {
	blob := "BLOB"
	if s.dialect == sqldialect.Postgres {
		blob = "BYTEA"
	}
	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (sha256 TEXT PRIMARY KEY, size BIGINT NOT NULL,
		data %s NOT NULL, created_at BIGINT NOT NULL)`, s.table, blob))
	return err
}

func (s *SQLCheckpointStore) Put(ctx context.Context, r io.Reader) (CheckpointRef, error) {
	var buf bytes.Buffer
	ref, err := hashTo(r, &buf, s.MaxBytes)
	if err != nil {
		return ref, err
	}
	_, err = s.db.ExecContext(ctx, s.dialect.Rebind(fmt.Sprintf(`INSERT INTO %s (sha256, size, data, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (sha256) DO NOTHING`, s.table)), ref.SHA256, ref.Size, buf.Bytes(), time.Now().Unix())
	return ref, err
}

func (s *SQLCheckpointStore) Open(ctx context.Context, ref CheckpointRef) (io.ReadCloser, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	var b []byte
	err := s.db.QueryRowContext(ctx, s.dialect.Rebind(fmt.Sprintf(`SELECT data FROM %s WHERE sha256 = ?`, s.table)), ref.SHA256).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCheckpointNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := verifyBytes(b, ref); err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (s *SQLCheckpointStore) Delete(ctx context.Context, ref CheckpointRef) error {
	_, err := s.db.ExecContext(ctx, s.dialect.Rebind(fmt.Sprintf(`DELETE FROM %s WHERE sha256 = ?`, s.table)), ref.SHA256)
	return err
}

// captureToStore archives dir into store.
func captureToStore(ctx context.Context, dir string, store CheckpointStore, limits ArchiveLimits) (CheckpointRef, error) {
	pr, pw := io.Pipe()
	go func() { pw.CloseWithError(CaptureDir(dir, pw, limits)) }()
	ref, err := store.Put(ctx, pr)
	pr.CloseWithError(err)
	return ref, err
}
