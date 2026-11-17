package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dixie6502/folio/internal/testutil"
)

// folio runs the command line in dir with stdin and returns its exit code,
// stdout and stderr.
func folio(t *testing.T, dir, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Chdir(dir)
	var stdout, stderr strings.Builder
	code := run(args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// tempHome gives the test a fresh, isolated home and returns it.
var tempHome = testutil.Home

// writeFile creates the empty file path, making its parents.
func writeFile(t *testing.T, path string) { testutil.Write(t, path, "") }

func TestInitAndDir(t *testing.T) {
	topic := filepath.Join(tempHome(t), "scratch", "topic")
	if err := os.MkdirAll(topic, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(topic, "notes") + "\n"

	if code, _, stderr := folio(t, topic, "", "--dir"); code != 1 || !strings.Contains(stderr, "folio init") {
		t.Fatalf("--dir before init: %d %q", code, stderr)
	}
	if code, _, stderr := folio(t, topic, "y\n", "init"); code != 1 || !strings.Contains(stderr, "--local") {
		t.Fatalf("init without a terminal must not ask: %d %q", code, stderr)
	}
	if code, stdout, _ := folio(t, topic, "", "init", "--local"); code != 0 || stdout != want {
		t.Fatalf("init --local: %d %q", code, stdout)
	}
	if code, stdout, _ := folio(t, topic, "", "--dir"); code != 0 || stdout != want {
		t.Fatalf("--dir: %d %q", code, stdout)
	}
}

func TestForgetRefusesWithoutATerminal(t *testing.T) {
	home := tempHome(t)
	owner := filepath.Join(home, "GitHub", "owner")
	writeFile(t, filepath.Join(owner, "notebooks", ".folio"))
	notebook := filepath.Join(owner, "notebooks", "gone")
	writeFile(t, filepath.Join(notebook, ".notebook"))

	if code, _, stderr := folio(t, owner, "y\n", "forget", "gone"); code != 1 || !strings.Contains(stderr, "needs a terminal") {
		t.Fatalf("forget with a piped yes: %d %q", code, stderr)
	}
	if _, err := os.Stat(notebook); err != nil {
		t.Fatalf("a piped yes deleted the notebook: %v", err)
	}
}

func TestCheckExitCode(t *testing.T) {
	home := tempHome(t)
	if code, _, _ := folio(t, home, "", "check"); code != 0 {
		t.Fatalf("check with no store: %d", code)
	}
	writeFile(t, filepath.Join(home, "GitHub", "owner", "notebooks", ".folio"))
	if code, stdout, _ := folio(t, home, "", "check"); code != 1 || !strings.Contains(stdout, "not a git repository") {
		t.Fatalf("check with a store outside git: %d %q", code, stdout)
	}
}

func TestSyncReportsEachStore(t *testing.T) {
	home := tempHome(t)
	store := filepath.Join(home, "GitHub", "owner", "notebooks")
	writeFile(t, filepath.Join(store, ".folio"))

	if code, stdout, _ := folio(t, home, "", "sync"); code != 0 || stdout != store+": skipped: not a git repository\n" {
		t.Fatalf("sync: %d %q", code, stdout)
	}
}

func TestUsage(t *testing.T) {
	home := tempHome(t)
	if code, stdout, _ := folio(t, home, "", "help"); code != 0 || !strings.Contains(stdout, "usage:") {
		t.Fatalf("help: %d %q", code, stdout)
	}
	if code, _, stderr := folio(t, home, "", "init", "extra"); code != 1 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("bad arguments: %d %q", code, stderr)
	}
}
