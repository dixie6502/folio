// Command folio is a notational-velocity switcher over one notebook per
// folder. This Go rewrite is in progress: it has the notebook commands; the
// switcher and sync still come from the Bash folio (see migration.md).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dixie6502/folio/internal/notebook"
)

const usage = `usage:
    folio --dir                print the current notebook's folder
    folio init [--local]       create the current folder's notebook
    folio mv <old> <new>       move a folder and its notebook
    folio forget <folder>      delete an orphan's notebook, after confirmation
    folio check                report problems; exit 1 if any
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
	input := bufio.NewReader(stdin)
	ask := func(question string) bool { return confirm(input, stderr, question) }

	var result string
	switch {
	case command == "--dir" && len(rest) == 0:
		result, err = env.Find(cwd)
	case command == "init" && len(rest) == 0:
		var interactive notebook.Confirm
		if isTerminal(stdin) {
			interactive = ask
		}
		result, err = env.Init(cwd, false, interactive)
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
		problems := env.Check()
		printLines(stdout, problems)
		if len(problems) > 0 {
			return 1
		}
		return 0
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
