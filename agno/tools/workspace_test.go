package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newTestWorkspace builds a workspace over a temp dir populated with a small tree.
func newTestWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()

	root := t.TempDir()
	// The temp dir itself may sit behind a symlink (/tmp -> /private/tmp on macOS),
	// so compare against the resolved path.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}

	mustWrite(t, filepath.Join(resolved, "main.go"), "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	mustWrite(t, filepath.Join(resolved, "README.md"), "# Title\n\nsome documentation here\n")
	mustMkdir(t, filepath.Join(resolved, "pkg"))
	mustWrite(t, filepath.Join(resolved, "pkg", "util.go"), "package pkg\n\nconst Answer = 42\n")
	mustMkdir(t, filepath.Join(resolved, ".git"))
	mustWrite(t, filepath.Join(resolved, ".git", "config"), "secret repo config\n")
	mustMkdir(t, filepath.Join(resolved, "node_modules"))
	mustWrite(t, filepath.Join(resolved, "node_modules", "index.js"), "module.exports = {}\n")

	ws, err := NewWorkspace(resolved)
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	return ws, resolved
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func readFile(t *testing.T, ws *Workspace, path string) *WorkspaceFileContent {
	t.Helper()
	res, err := ws.ReadFile(WorkspaceReadFileParams{Path: path})
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	content, ok := res.(*WorkspaceFileContent)
	if !ok {
		t.Fatalf("ReadFile returned %T, want *WorkspaceFileContent", res)
	}
	return content
}

// --- Guard-rails: path sandbox ---

func TestPathTraversalIsBlocked(t *testing.T) {
	ws, root := newTestWorkspace(t)

	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	mustWrite(t, outside, "secret\n")
	t.Cleanup(func() { os.Remove(outside) })

	traversals := []string{
		"../outside.txt",
		"../../etc/passwd",
		"pkg/../../outside.txt",
		"./pkg/../../../etc/hosts",
		outside,       // absolute path outside the root
		"/etc/passwd", // absolute path outside the root
	}

	for _, path := range traversals {
		t.Run(path, func(t *testing.T) {
			if _, err := ws.ReadFile(WorkspaceReadFileParams{Path: path}); err == nil {
				t.Errorf("ReadFile(%q) should be rejected, got nil error", path)
			}
			if _, err := ws.WriteFile(WorkspaceWriteFileParams{Path: path, Content: "owned", Overwrite: true}); err == nil {
				t.Errorf("WriteFile(%q) should be rejected, got nil error", path)
			}
			if _, err := ws.DeleteFile(WorkspaceDeleteFileParams{Path: path}); err == nil {
				t.Errorf("DeleteFile(%q) should be rejected, got nil error", path)
			}
			if _, err := ws.ListFiles(WorkspaceListFilesParams{Directory: path}); err == nil {
				t.Errorf("ListFiles(%q) should be rejected, got nil error", path)
			}
		})
	}

	// The file outside the workspace must be untouched.
	data, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("the file outside the workspace was removed: %v", err)
	}
	if string(data) != "secret\n" {
		t.Fatalf("the file outside the workspace was modified: %q", data)
	}
}

func TestSymlinkEscapeIsBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require extra privileges on Windows")
	}

	ws, root := newTestWorkspace(t)

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	mustWrite(t, outsideFile, "top secret\n")

	// A symlink to a file outside the workspace, and one to a directory outside it.
	if err := os.Symlink(outsideFile, filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(root, "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := ws.ReadFile(WorkspaceReadFileParams{Path: "link.txt"}); err == nil {
		t.Error("reading through a symlink pointing outside the workspace should be rejected")
	}
	if _, err := ws.ReadFile(WorkspaceReadFileParams{Path: "escape/secret.txt"}); err == nil {
		t.Error("reading through a symlinked directory pointing outside the workspace should be rejected")
	}
	// Writing a new file through the symlinked directory must not escape either.
	if _, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "escape/planted.txt", Content: "x"}); err == nil {
		t.Error("writing through a symlinked directory pointing outside the workspace should be rejected")
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "planted.txt")); err == nil {
		t.Error("a file was planted outside the workspace root")
	}
}

// --- Guard-rails: read before write ---

func TestWriteWithoutReadIsBlocked(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	_, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "main.go", Content: "package main\n", Overwrite: true})
	if err == nil {
		t.Fatal("overwriting a file that was never read should be rejected")
	}
	if !strings.Contains(err.Error(), "read_file") {
		t.Errorf("the error should tell the caller to read the file first, got: %v", err)
	}

	// Reading it unlocks the write.
	readFile(t, ws, "main.go")
	if _, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "main.go", Content: "package main\n", Overwrite: true}); err != nil {
		t.Fatalf("write after read should succeed: %v", err)
	}
}

func TestWriteRequiresOverwriteFlag(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	readFile(t, ws, "README.md")

	if _, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "README.md", Content: "new"}); err == nil {
		t.Fatal("overwriting an existing file without overwrite=true should be rejected")
	}
}

func TestWriteNewFileIsAllowed(t *testing.T) {
	ws, root := newTestWorkspace(t)

	res, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "sub/dir/new.txt", Content: "hello\n"})
	if err != nil {
		t.Fatalf("creating a new file should be allowed: %v", err)
	}
	if result := res.(*WorkspaceWriteResult); !result.Success {
		t.Errorf("write result should be successful, got %+v", result)
	}

	data, err := os.ReadFile(filepath.Join(root, "sub", "dir", "new.txt"))
	if err != nil {
		t.Fatalf("the new file was not created: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("content = %q, want %q", data, "hello\n")
	}
}

func TestEditWithoutReadIsBlocked(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	_, err := ws.EditFile(WorkspaceEditFileParams{Path: "main.go", OldString: "hello", NewString: "world"})
	if err == nil {
		t.Fatal("editing a file that was never read should be rejected")
	}

	readFile(t, ws, "main.go")
	if _, err := ws.EditFile(WorkspaceEditFileParams{Path: "main.go", OldString: "hello", NewString: "world"}); err != nil {
		t.Fatalf("edit after read should succeed: %v", err)
	}
}

func TestDeleteAndMoveWithoutReadAreBlocked(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	if _, err := ws.DeleteFile(WorkspaceDeleteFileParams{Path: "README.md"}); err == nil {
		t.Error("deleting a file that was never read should be rejected")
	}
	if _, err := ws.MoveFile(WorkspaceMoveFileParams{Src: "README.md", Dst: "DOCS.md"}); err == nil {
		t.Error("moving a file that was never read should be rejected")
	}

	readFile(t, ws, "README.md")
	if _, err := ws.MoveFile(WorkspaceMoveFileParams{Src: "README.md", Dst: "DOCS.md"}); err != nil {
		t.Fatalf("move after read should succeed: %v", err)
	}
	if _, err := ws.DeleteFile(WorkspaceDeleteFileParams{Path: "DOCS.md"}); err != nil {
		t.Fatalf("delete after read (of the moved file) should succeed: %v", err)
	}
}

func TestAllowBlindWritesDisablesReadBeforeWrite(t *testing.T) {
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	mustWrite(t, filepath.Join(resolved, "a.txt"), "old\n")

	ws, err := NewWorkspaceWithConfig(WorkspaceConfig{Root: resolved, AllowBlindWrites: true})
	if err != nil {
		t.Fatalf("NewWorkspaceWithConfig: %v", err)
	}

	if _, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "a.txt", Content: "new\n", Overwrite: true}); err != nil {
		t.Fatalf("with AllowBlindWrites the write should succeed: %v", err)
	}
}

// --- Guard-rails: size limit ---

func TestMaxFileSizeIsEnforced(t *testing.T) {
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	mustWrite(t, filepath.Join(resolved, "big.txt"), strings.Repeat("x", 200))

	ws, err := NewWorkspaceWithConfig(WorkspaceConfig{Root: resolved, MaxFileSize: 100})
	if err != nil {
		t.Fatalf("NewWorkspaceWithConfig: %v", err)
	}

	if _, err := ws.ReadFile(WorkspaceReadFileParams{Path: "big.txt"}); err == nil {
		t.Error("reading a file over the size limit should be rejected")
	}
	if _, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "other.txt", Content: strings.Repeat("y", 200)}); err == nil {
		t.Error("writing content over the size limit should be rejected")
	}
}

// --- Operations ---

func TestReadFileAddsLineNumbers(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	content := readFile(t, ws, "pkg/util.go")
	if content.TotalLines != 3 {
		t.Errorf("TotalLines = %d, want 3", content.TotalLines)
	}
	if content.Truncated {
		t.Error("the file should not be reported as truncated")
	}
	if !strings.Contains(content.Content, "     1\tpackage pkg") {
		t.Errorf("content should be line-numbered, got:\n%s", content.Content)
	}
	if !strings.Contains(content.Content, "     3\tconst Answer = 42") {
		t.Errorf("content should keep the source line numbers, got:\n%s", content.Content)
	}
}

func TestReadFileRangeKeepsSourceLineNumbers(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	res, err := ws.ReadFile(WorkspaceReadFileParams{Path: "main.go", StartLine: 3, Limit: 1})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := res.(*WorkspaceFileContent)

	if content.StartLine != 3 || content.EndLine != 3 {
		t.Errorf("range = %d-%d, want 3-3", content.StartLine, content.EndLine)
	}
	if !content.Truncated {
		t.Error("a partial read should be reported as truncated")
	}
	if !strings.HasPrefix(strings.TrimSpace(content.Content), "3\t") {
		t.Errorf("the chunk should start at line 3, got: %q", content.Content)
	}

	// A partial read does not count as having read the file.
	if _, err := ws.WriteFile(WorkspaceWriteFileParams{Path: "main.go", Content: "x", Overwrite: true}); err == nil {
		t.Error("a partial read should not unlock a write")
	}
}

func TestListFilesSkipsExcludedDirectories(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	res, err := ws.ListFiles(WorkspaceListFilesParams{Recursive: true})
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	listing := res.(*WorkspaceListing)

	for _, entry := range listing.Entries {
		if strings.HasPrefix(entry.Path, ".git") || strings.HasPrefix(entry.Path, "node_modules") {
			t.Errorf("excluded path %q was listed", entry.Path)
		}
	}

	found := map[string]bool{}
	for _, entry := range listing.Entries {
		found[entry.Path] = true
	}
	for _, want := range []string{"main.go", "README.md", "pkg", filepath.Join("pkg", "util.go")} {
		if !found[want] {
			t.Errorf("expected %q in the listing, got %v", want, listing.Entries)
		}
	}
}

func TestListFilesPattern(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	res, err := ws.ListFiles(WorkspaceListFilesParams{Recursive: true, Pattern: "*.go"})
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	listing := res.(*WorkspaceListing)

	for _, entry := range listing.Entries {
		if !entry.IsDir && !strings.HasSuffix(entry.Path, ".go") {
			t.Errorf("entry %q does not match the pattern *.go", entry.Path)
		}
	}
}

func TestSearchContentReturnsSnippets(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	res, err := ws.SearchContent(WorkspaceSearchContentParams{Query: "DOCUMENTATION"})
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	result := res.(*WorkspaceSearchResult)

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (matches: %+v)", result.Count, result.Matches)
	}
	match := result.Matches[0]
	if match.Path != "README.md" {
		t.Errorf("Path = %q, want README.md", match.Path)
	}
	if match.Line != 3 {
		t.Errorf("Line = %d, want 3", match.Line)
	}
	if !strings.Contains(match.Snippet, "some documentation here") {
		t.Errorf("Snippet = %q, should contain the matching text", match.Snippet)
	}
}

func TestSearchContentSkipsExcludedDirectories(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	res, err := ws.SearchContent(WorkspaceSearchContentParams{Query: "secret repo config"})
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	if result := res.(*WorkspaceSearchResult); result.Count != 0 {
		t.Errorf("search should not descend into .git, got %+v", result.Matches)
	}
}

func TestEditFileRequiresUniqueMatch(t *testing.T) {
	ws, root := newTestWorkspace(t)
	mustWrite(t, filepath.Join(root, "dup.txt"), "a\na\nb\n")
	readFile(t, ws, "dup.txt")

	_, err := ws.EditFile(WorkspaceEditFileParams{Path: "dup.txt", OldString: "a", NewString: "c"})
	if err == nil {
		t.Fatal("an ambiguous edit should be rejected")
	}
	if !strings.Contains(err.Error(), "replace_all") {
		t.Errorf("the error should mention replace_all, got: %v", err)
	}

	res, err := ws.EditFile(WorkspaceEditFileParams{Path: "dup.txt", OldString: "a", NewString: "c", ReplaceAll: true})
	if err != nil {
		t.Fatalf("replace_all edit should succeed: %v", err)
	}
	if result := res.(*WorkspaceWriteResult); result.Replaced != 2 {
		t.Errorf("Replaced = %d, want 2", result.Replaced)
	}

	data, _ := os.ReadFile(filepath.Join(root, "dup.txt"))
	if string(data) != "c\nc\nb\n" {
		t.Errorf("content = %q, want %q", data, "c\nc\nb\n")
	}
}

func TestEditFileMissingOldString(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	readFile(t, ws, "main.go")

	if _, err := ws.EditFile(WorkspaceEditFileParams{Path: "main.go", OldString: "not-there", NewString: "x"}); err == nil {
		t.Fatal("editing with an old_string that is not in the file should be rejected")
	}
}

func TestRunCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test command is POSIX-only")
	}
	ws, _ := newTestWorkspace(t)

	res, err := ws.RunCommand(WorkspaceRunCommandParams{Command: "pwd"})
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	result := res.(*WorkspaceCommandResult)

	if !result.Success || result.ExitCode != 0 {
		t.Errorf("command should succeed, got %+v", result)
	}
	if strings.TrimSpace(result.Stdout) != ws.Root() {
		t.Errorf("the command should run in the workspace root, got %q want %q", result.Stdout, ws.Root())
	}
}

func TestRunCommandNonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test command is POSIX-only")
	}
	ws, _ := newTestWorkspace(t)

	res, err := ws.RunCommand(WorkspaceRunCommandParams{Command: "false"})
	if err != nil {
		t.Fatalf("a non-zero exit should be reported in the result, not as an error: %v", err)
	}
	result := res.(*WorkspaceCommandResult)
	if result.Success || result.ExitCode == 0 {
		t.Errorf("the command should be reported as failed, got %+v", result)
	}
}

func TestRunCommandNotFound(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	if _, err := ws.RunCommand(WorkspaceRunCommandParams{Command: "definitely-not-a-command-xyz"}); err == nil {
		t.Fatal("a command that cannot be started should return an error")
	}
}

// --- Registration ---

func TestDefaultRegistrationAndConfirmations(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	// The toolkit keys its methods as "<ToolkitName>_<method>".
	methods := ws.GetMethods()
	want := []string{"read_file", "list_files", "search_content", "write_file", "edit_file", "move_file", "delete_file", "run_command"}
	for _, name := range want {
		if _, ok := methods["Workspace_"+name]; !ok {
			t.Errorf("method %q should be registered by default", name)
		}
	}

	for _, name := range []string{"write_file", "edit_file", "move_file", "delete_file", "run_command"} {
		if !methods["Workspace_"+name].RequiresConfirmation {
			t.Errorf("destructive method %q should require confirmation by default", name)
		}
	}
	for _, name := range []string{"read_file", "list_files", "search_content"} {
		if methods["Workspace_"+name].RequiresConfirmation {
			t.Errorf("read-only method %q should not require confirmation", name)
		}
	}
}

func TestAllowedOperationsRestrictTheSurface(t *testing.T) {
	ws, err := NewWorkspaceWithConfig(WorkspaceConfig{
		Root:    t.TempDir(),
		Allowed: []string{OpRead, OpList, OpSearch},
		Confirm: []string{},
	})
	if err != nil {
		t.Fatalf("NewWorkspaceWithConfig: %v", err)
	}

	methods := ws.GetMethods()
	if len(methods) != 3 {
		t.Errorf("only the read operations should be registered, got %d methods", len(methods))
	}
	for _, name := range []string{"write_file", "edit_file", "move_file", "delete_file", "run_command"} {
		if _, ok := methods["Workspace_"+name]; ok {
			t.Errorf("method %q should not be registered", name)
		}
	}
	for _, name := range []string{"read_file", "list_files", "search_content"} {
		if methods["Workspace_"+name].RequiresConfirmation {
			t.Errorf("method %q should not require confirmation when Confirm is empty", name)
		}
	}
}

func TestUnknownOperationIsRejected(t *testing.T) {
	_, err := NewWorkspaceWithConfig(WorkspaceConfig{Root: t.TempDir(), Allowed: []string{"read", "nuke"}})
	if err == nil {
		t.Fatal("an unknown operation alias should be rejected")
	}
	if !strings.Contains(err.Error(), "nuke") {
		t.Errorf("the error should name the unknown operation, got: %v", err)
	}
}

func TestNewWorkspaceRejectsBadRoot(t *testing.T) {
	if _, err := NewWorkspace(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("a root that does not exist should be rejected")
	}

	file := filepath.Join(t.TempDir(), "file.txt")
	mustWrite(t, file, "x")
	if _, err := NewWorkspace(file); err == nil {
		t.Error("a root that is not a directory should be rejected")
	}
}
