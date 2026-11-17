// Command folio is a notational-velocity switcher over one notebook per
// folder. This Go rewrite is in progress: it has the notebook commands and
// sync; the switcher still comes from the Bash folio (see migration.md).
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dixie6502/folio/internal/notebook"
	"github.com/dixie6502/folio/internal/storesync"
)

const usage = `usage:
    folio --dir                print the current notebook's folder
    folio init [--local]       create the current folder's notebook
    folio mv <old> <new>       move a folder and its notebook
    folio forget <folder>      delete an orphan's notebook, after confirmation
    folio check                report problems; exit 1 if any
    folio sync                 sync every store with its remote, now
    folio sync --daemon        keep syncing (run by the folio-sync service)
    folio help                 show this message
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the command in args and returns the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	env, err := notebook.NewEnv()
	if err != nil {
		return fail(stderr, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, err)
	}
	command, rest := "", []string(nil)
	if len(args) > 0 {
		command, rest = args[0], args[1:]
	}
	// Only a person at a terminal answers questions: a calling tool never
	// answers on the user's behalf.
	var ask notebook.Confirm
	if isTerminal(stdin) {
		input := bufio.NewReader(stdin)
		ask = func(question string) bool { return confirm(input, stderr, question) }
	}

	var result string
	switch {
	case command == "--dir" && len(rest) == 0:
		result, err = env.Find(cwd)
	case command == "init" && len(rest) == 0:
		result, err = env.Init(cwd, false, ask)
	case command == "init" && len(rest) == 1 && rest[0] == "--local":
		result, err = env.Init(cwd, true, nil)
	case command == "forget" && len(rest) == 1:
		result, err = env.Forget(rest[0], ask)
	case command == "mv" && len(rest) == 2:
		done, err := env.Move(rest[0], rest[1])
		printLines(stdout, done)
		if err != nil {
			return fail(stderr, err)
		}
		return 0
	case command == "check" && len(rest) == 0:
		return check(env, stdout)
	case command == "sync" && len(rest) == 0:
		return syncNow(env, stdout, stderr)
	case command == "sync" && len(rest) == 1 && rest[0] == "--daemon":
		return syncDaemon(env, stderr)
	case command == "help" || command == "-h" || command == "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprint(stderr, usage)
		return fail(stderr, errors.New("unknown command or arguments: "+strings.Join(args, " ")))
	}
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, result)
	return 0
}

// check prints the problems of the notebooks, then of their sync; it returns
// 1 when there is any. A sync check that cannot run is itself reported, after
// the notebook problems.
func check(env notebook.Env, stdout io.Writer) int {
	problems := env.Check()
	stores, _ := env.Stores()
	syncer, err := storesync.New()
	if err == nil {
		var syncProblems []string
		syncProblems, err = syncer.Check(stores)
		problems = append(problems, syncProblems...)
	}
	if err != nil {
		problems = append(problems, "sync status unavailable: "+err.Error())
	}
	printLines(stdout, problems)
	if len(problems) > 0 {
		return 1
	}
	return 0
}

// syncNow syncs every store once, printing each outcome; it returns 1 when
// any store failed.
func syncNow(env notebook.Env, stdout, stderr io.Writer) int {
	syncer, err := storesync.New()
	if err != nil {
		return fail(stderr, err)
	}
	code := 0
	stores, _ := env.Stores()
	for _, store := range stores {
		outcome, err := syncer.Store(store)
		if err != nil {
			code = fail(stderr, fmt.Errorf("%s: %w", store, err))
			continue
		}
		fmt.Fprintf(stdout, "%s: %s\n", store, outcome)
	}
	return code
}

// syncDaemon keeps every store synced until interrupted or terminated.
func syncDaemon(env notebook.Env, stderr io.Writer) int {
	syncer, err := storesync.New()
	if err != nil {
		return fail(stderr, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stores := func() []string {
		list, _ := env.Stores()
		return list
	}
	syncer.Daemon(ctx, stores, func(format string, args ...any) {
		fmt.Fprintf(stderr, format+"\n", args...)
	})
	return 0
}

// printLines prints each line on stdout.
func printLines(stdout io.Writer, lines []string) {
	for _, line := range lines {
		fmt.Fprintln(stdout, line)
	}
}

// fail prints err and returns the failure exit code.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "folio:", err)
	return 1
}

// confirm asks question on stderr and reads a yes or no from input.
func confirm(input *bufio.Reader, stderr io.Writer, question string) bool {
	fmt.Fprintf(stderr, "%s [y/N] ", question)
	answer, _ := input.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// isTerminal reports whether reader is an interactive terminal.
func isTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
