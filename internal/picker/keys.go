package picker

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// KeyKind names what a key does.
type KeyKind int

// The keys the picker knows.
const (
	KeyRune       KeyKind = iota // a character typed: Key.Text
	KeyPaste                     // text pasted: Key.Text
	KeyEnter                     // Enter (Ctrl-M)
	KeyBackspace                 // Backspace (Ctrl-H)
	KeyClear                     // Ctrl-U: clear the query
	KeyDeleteWord                // Ctrl-W: delete the query's last word
	KeyUp                        // Up, Ctrl-K, Ctrl-P
	KeyDown                      // Down, Ctrl-J
	KeyCreate                    // Ctrl-N
	KeyCancel                    // Esc, Ctrl-C, Ctrl-G
)

// Key is one decoded key press.
type Key struct {
	Kind KeyKind
	Text string // for KeyRune and KeyPaste
}

// Bracketed paste markers, sent around pasted text once enabled.
const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// decoder turns terminal input into keys. It keeps a paste that spans reads.
type decoder struct {
	paste   *strings.Builder // the paste being read, nil outside one
	pending []byte           // an incomplete UTF-8 character or paste marker
}

// feed decodes input, read from the terminal at once, into keys.
func (d *decoder) feed(input []byte) []Key {
	input = append(d.pending, input...)
	d.pending = nil
	var keys []Key
	for len(input) > 0 {
		if d.paste != nil {
			end := bytes.Index(input, []byte(pasteEnd))
			if end < 0 {
				keep := partialSuffix(input, pasteEnd)
				d.paste.Write(input[:len(input)-keep])
				d.pending = input[len(input)-keep:]
				return keys
			}
			d.paste.Write(input[:end])
			keys = append(keys, Key{Kind: KeyPaste, Text: d.paste.String()})
			d.paste, input = nil, input[end+len(pasteEnd):]
			continue
		}
		if bytes.HasPrefix(input, []byte(pasteStart)) {
			d.paste, input = &strings.Builder{}, input[len(pasteStart):]
			continue
		}
		if len(input) >= 2 && len(input) < len(pasteStart) && strings.HasPrefix(pasteStart, string(input)) {
			d.pending = input // a paste marker split across reads
			return keys
		}
		key, size, isKey := decodeOne(input)
		if size == 0 { // an incomplete character: wait for the rest
			d.pending = input
			return keys
		}
		if isKey {
			keys = append(keys, key)
		}
		input = input[size:]
	}
	return keys
}

// singleByteKeys are the keys a terminal sends as one control byte.
var singleByteKeys = map[byte]KeyKind{
	'\r': KeyEnter, '\n': KeyDown, 0x7f: KeyBackspace, 0x08: KeyBackspace, 0x15: KeyClear,
	0x17: KeyDeleteWord, 0x0b: KeyUp, 0x10: KeyUp, 0x0e: KeyCreate, 0x03: KeyCancel, 0x07: KeyCancel,
}

// decodeOne decodes the key at the start of input and its length; isKey is
// false for input that means no key, and a length of 0 means input holds
// only part of a character.
func decodeOne(input []byte) (key Key, size int, isKey bool) {
	b := input[0]
	if kind, ok := singleByteKeys[b]; ok {
		return Key{Kind: kind}, 1, true
	}
	switch {
	case b == 0x1b:
		return decodeEscape(input)
	case b < 0x20:
		return Key{}, 1, false
	case !utf8.FullRune(input):
		return Key{}, 0, false
	}
	r, size := utf8.DecodeRune(input)
	return Key{Kind: KeyRune, Text: string(r)}, size, r != utf8.RuneError
}

// decodeEscape decodes an escape sequence: an arrow, or a lone Esc (nothing
// follows it in the same read). Other sequences (function keys, mouse) are
// skipped.
func decodeEscape(input []byte) (key Key, size int, isKey bool) {
	if len(input) == 1 || input[1] != '[' && input[1] != 'O' {
		return Key{Kind: KeyCancel}, 1, true // Esc, then any next key on its own
	}
	// CSI or SS3: parameters, then one final byte in @..~.
	for i := 2; i < len(input); i++ {
		if input[i] >= 0x40 && input[i] <= 0x7e {
			switch input[i] {
			case 'A':
				return Key{Kind: KeyUp}, i + 1, true
			case 'B':
				return Key{Kind: KeyDown}, i + 1, true
			}
			return Key{}, i + 1, false
		}
	}
	return Key{}, len(input), false
}

// partialSuffix returns how many bytes at the end of input could begin
// marker, so a marker split across reads is still found.
func partialSuffix(input []byte, marker string) int {
	for keep := min(len(marker)-1, len(input)); keep > 0; keep-- {
		if bytes.HasSuffix([]byte(marker[:keep]), input[len(input)-keep:]) {
			return keep
		}
	}
	return 0
}
