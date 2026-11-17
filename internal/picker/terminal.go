package picker

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dixie6502/folio/internal/notes"
	"golang.org/x/term"
)

// Terminal control sequences.
const (
	clearLine     = "\x1b[2K"
	clearBelow    = "\x1b[J"
	pasteOn       = "\x1b[?2004h"
	pasteOff      = "\x1b[?2004l"
	cursorUpFmt   = "\x1b[%dA"
	cursorMoveFmt = "\x1b[%dC"
)

// Run shows the picker over list on the terminal (/dev/tty, so it works
// whatever stdin and stdout are) until the user chooses, and returns the
// choice. It draws inline, below the prompt, and erases itself on return.
func Run(list []notes.Note, mode Mode, query string) (Result, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return Result{}, fmt.Errorf("no terminal for the picker: %w", err)
	}
	defer func() { _ = tty.Close() }() // also ends the reader goroutine
	fd := int(tty.Fd())
	saved, err := term.MakeRaw(fd)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = term.Restore(fd, saved) }()

	screen := &screen{out: bufio.NewWriter(tty), fd: fd}
	screen.write(pasteOn)
	defer screen.close()

	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	input := make(chan []byte)
	go read(tty, input)

	s := newState(list, mode, query)
	screen.draw(s)
	var keys decoder
	for {
		select {
		case <-resized:
			screen.draw(s)
		case chunk, ok := <-input:
			if !ok {
				return Result{}, fmt.Errorf("terminal closed")
			}
			for _, key := range keys.feed(chunk) {
				if done, result := s.handle(key); done {
					return result, nil
				}
			}
			screen.draw(s)
		}
	}
}

// read sends what the terminal sends until it fails (as when it is closed).
func read(tty *os.File, input chan<- []byte) {
	defer close(input)
	buffer := make([]byte, 4096)
	for {
		n, err := tty.Read(buffer)
		if err != nil {
			return
		}
		input <- append([]byte(nil), buffer[:n]...)
	}
}

// screen draws the picker in a block of lines below the cursor.
type screen struct {
	out    *bufio.Writer
	fd     int
	height int // lines reserved so far
}

// draw redraws the picker; on a new height (a resize) it first erases the
// old block and reserves the new one.
func (sc *screen) draw(s *state) {
	width, rows, err := term.GetSize(sc.fd)
	if err != nil || width < 4 || rows < 3 {
		width, rows = 80, 24
	}
	height := s.height(rows)
	if height != sc.height {
		sc.write("\r" + clearBelow + strings.Repeat("\r\n", height-1))
		if height > 1 {
			sc.write(fmt.Sprintf(cursorUpFmt, height-1))
		}
		sc.height = height
	}
	lines, column := s.lines(width, height)
	sc.write("\r")
	for i, line := range lines {
		sc.write(clearLine + line)
		if i < len(lines)-1 {
			sc.write("\r\n")
		}
	}
	if height > 1 {
		sc.write(fmt.Sprintf(cursorUpFmt, height-1))
	}
	sc.write("\r")
	if column > 0 {
		sc.write(fmt.Sprintf(cursorMoveFmt, column))
	}
	_ = sc.out.Flush()
}

// close erases the picker and leaves the cursor where it started.
func (sc *screen) close() {
	sc.write("\r" + clearBelow + pasteOff)
	_ = sc.out.Flush()
}

// write queues text for the terminal.
func (sc *screen) write(text string) { _, _ = sc.out.WriteString(text) }
