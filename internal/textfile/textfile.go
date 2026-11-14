// Package textfile turns the bytes of a file into the text the editor shows
// and back: it finds the file's encoding and line endings, decodes it to a
// string with "\n" line endings, and encodes text again in the encoding and
// line endings chosen. It has no dependency on Qt.
//
// The encodings are those Qt has built in (UTF-8, UTF-16, UTF-32 and
// Latin-1), done here in Go so that they can be tested without a GUI.
package textfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Encoding is a character encoding, by its id as the config file and menus
// know it.
type Encoding string

// The encodings the editor reads and writes. UTF-16 and UTF-32 are always
// written with a byte order mark, which is how other programs tell them
// apart; UTF-8 has one only when it is asked for (UTF8BOM).
const (
	UTF8    Encoding = "utf-8"
	UTF8BOM Encoding = "utf-8-bom"
	UTF16LE Encoding = "utf-16le"
	UTF16BE Encoding = "utf-16be"
	UTF32LE Encoding = "utf-32le"
	UTF32BE Encoding = "utf-32be"
	Latin1  Encoding = "iso-8859-1"
)

// Encodings are every encoding, in the order menus offer them.
var Encodings = []Encoding{UTF8, UTF8BOM, UTF16LE, UTF16BE, UTF32LE, UTF32BE, Latin1}

var encodingNames = map[Encoding]string{
	UTF8:    "UTF-8",
	UTF8BOM: "UTF-8 with BOM",
	UTF16LE: "UTF-16 LE",
	UTF16BE: "UTF-16 BE",
	UTF32LE: "UTF-32 LE",
	UTF32BE: "UTF-32 BE",
	Latin1:  "ISO-8859-1",
}

// Name returns the encoding's name as people read it ("UTF-16 LE").
func (e Encoding) Name() string {
	if n, ok := encodingNames[e]; ok {
		return n
	}
	return string(e)
}

// The byte order marks.
var (
	bomUTF8    = []byte{0xef, 0xbb, 0xbf}
	bomUTF16LE = []byte{0xff, 0xfe}
	bomUTF16BE = []byte{0xfe, 0xff}
	bomUTF32LE = []byte{0xff, 0xfe, 0, 0}
	bomUTF32BE = []byte{0, 0, 0xfe, 0xff}
)

// bom returns the byte order mark the encoding is written with, if any.
func (e Encoding) bom() []byte {
	switch e {
	case UTF8BOM:
		return bomUTF8
	case UTF16LE:
		return bomUTF16LE
	case UTF16BE:
		return bomUTF16BE
	case UTF32LE:
		return bomUTF32LE
	case UTF32BE:
		return bomUTF32BE
	}
	return nil
}

// sniffLen is how much of a file is looked at to guess an encoding that
// has no byte order mark.
const sniffLen = 8192

// Detect guesses a file's encoding from its bytes: a byte order mark says
// which it is; otherwise text that is valid UTF-8 is UTF-8, text in which
// every other byte is zero is UTF-16, and anything else is Latin-1, which
// any bytes decode as.
func Detect(data []byte) Encoding {
	// UTF-32 LE's mark begins with UTF-16 LE's, so it is looked for first.
	switch {
	case bytes.HasPrefix(data, bomUTF32LE):
		return UTF32LE
	case bytes.HasPrefix(data, bomUTF32BE):
		return UTF32BE
	case bytes.HasPrefix(data, bomUTF8):
		return UTF8BOM
	case bytes.HasPrefix(data, bomUTF16LE):
		return UTF16LE
	case bytes.HasPrefix(data, bomUTF16BE):
		return UTF16BE
	}
	if e, ok := sniffUTF16(data); ok {
		return e
	}
	if utf8.Valid(data) {
		return UTF8
	}
	return Latin1
}

// sniffUTF16 recognises UTF-16 without a byte order mark by its zero bytes:
// in text that is mostly ASCII, as most is, every other byte is zero, on
// the odd bytes for little endian and the even ones for big endian.
func sniffUTF16(data []byte) (Encoding, bool) {
	n := min(len(data), sniffLen) &^ 1
	if n < 4 {
		return "", false
	}
	var even, odd int
	for i := 0; i < n; i += 2 {
		if data[i] == 0 {
			even++
		}
		if data[i+1] == 0 {
			odd++
		}
	}
	pairs := n / 2
	switch {
	case odd*10 >= pairs*7 && even*10 <= pairs:
		return UTF16LE, true
	case even*10 >= pairs*7 && odd*10 <= pairs:
		return UTF16BE, true
	}
	return "", false
}

// LooksBinary reports whether data, read as enc, seems not to be text at
// all: it has a NUL character near its start.
func LooksBinary(data []byte, enc Encoding) bool {
	switch enc {
	case UTF16LE, UTF16BE, UTF32LE, UTF32BE:
		text, err := Decode(data[:min(len(data), sniffLen)&^3], enc)
		return err == nil && strings.ContainsRune(text, 0)
	}
	return bytes.IndexByte(data[:min(len(data), sniffLen)], 0) >= 0
}

// Decode decodes data in the encoding, leaving out any byte order mark.
// UTF-8 that is not valid is an error, as decoding it would lose what the
// bad bytes were; in UTF-16 and UTF-32 a bad code unit, or a short one at the
// end, decodes as U+FFFD.
func Decode(data []byte, enc Encoding) (string, error) {
	data = bytes.TrimPrefix(data, enc.bom())
	switch enc {
	case UTF8, UTF8BOM:
		if !utf8.Valid(data) {
			return "", fmt.Errorf("the file is not valid %s", enc.Name())
		}
		return string(data), nil
	case UTF16LE, UTF16BE:
		return decodeUTF16(data, order(enc)), nil
	case UTF32LE, UTF32BE:
		return decodeUTF32(data, order(enc)), nil
	case Latin1:
		var b strings.Builder
		b.Grow(len(data))
		for _, c := range data {
			b.WriteRune(rune(c))
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("unknown encoding %q", enc)
}

// byteOrder reads and appends in one byte order.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

func order(enc Encoding) byteOrder {
	if enc == UTF16BE || enc == UTF32BE {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func decodeUTF16(data []byte, bo byteOrder) string {
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = bo.Uint16(data[2*i:])
	}
	var b strings.Builder
	b.Grow(len(data) / 2)
	for _, r := range utf16.Decode(units) {
		b.WriteRune(r)
	}
	if len(data)%2 != 0 {
		b.WriteRune(utf8.RuneError)
	}
	return b.String()
}

func decodeUTF32(data []byte, bo byteOrder) string {
	var b strings.Builder
	b.Grow(len(data) / 4)
	for i := 0; i+4 <= len(data); i += 4 {
		r := rune(bo.Uint32(data[i:]))
		if !utf8.ValidRune(r) {
			r = utf8.RuneError
		}
		b.WriteRune(r)
	}
	if len(data)%4 != 0 {
		b.WriteRune(utf8.RuneError)
	}
	return b.String()
}

// Encode encodes text in the encoding, with its byte order mark if it has
// one. Text with a character the encoding cannot hold (Latin-1 holds only
// the first 256) is an error that says which, and on which line.
func Encode(text string, enc Encoding) ([]byte, error) {
	out := append([]byte(nil), enc.bom()...)
	switch enc {
	case UTF8, UTF8BOM:
		return append(out, text...), nil
	case UTF16LE, UTF16BE:
		bo := order(enc)
		out = slices.Grow(out, len(text)*2)
		for _, r := range text {
			if r1, r2 := utf16.EncodeRune(r); r1 != utf8.RuneError || r2 != utf8.RuneError {
				out = bo.AppendUint16(out, uint16(r1))
				out = bo.AppendUint16(out, uint16(r2))
			} else {
				out = bo.AppendUint16(out, uint16(r))
			}
		}
		return out, nil
	case UTF32LE, UTF32BE:
		bo := order(enc)
		out = slices.Grow(out, len(text)*4)
		for _, r := range text {
			out = bo.AppendUint32(out, uint32(r))
		}
		return out, nil
	case Latin1:
		out = slices.Grow(out, len(text))
		line := 1
		for _, r := range text {
			if r > 0xff {
				return nil, fmt.Errorf("line %d has a character %s cannot hold: %q (U+%04X)", line, enc.Name(), r, r)
			}
			if r == '\n' {
				line++
			}
			out = append(out, byte(r))
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown encoding %q", enc)
}

// LineEnding is the way lines end in a file.
type LineEnding string

// The line endings.
const (
	LF   LineEnding = "unix"    // \n
	CRLF LineEnding = "windows" // \r\n
	CR   LineEnding = "mac"     // \r, as classic Mac OS had
)

// LineEndings are every line ending, in the order menus offer them.
var LineEndings = []LineEnding{LF, CRLF, CR}

// Name returns the line ending's name as people read it.
func (l LineEnding) Name() string {
	switch l {
	case LF:
		return "Unix (LF)"
	case CRLF:
		return "Windows (CRLF)"
	case CR:
		return "Classic Mac (CR)"
	}
	return string(l)
}

// Short returns a short name for the status bar.
func (l LineEnding) Short() string {
	switch l {
	case LF:
		return "Unix"
	case CRLF:
		return "Windows"
	case CR:
		return "Mac"
	}
	return string(l)
}

// Sequence returns the characters the line ending is written as.
func (l LineEnding) Sequence() string {
	switch l {
	case CRLF:
		return "\r\n"
	case CR:
		return "\r"
	}
	return "\n"
}

// DetectLineEnding returns the line ending text uses most, or LF for text
// with none.
func DetectLineEnding(text string) LineEnding {
	crlf := strings.Count(text, "\r\n")
	lf := strings.Count(text, "\n") - crlf
	cr := strings.Count(text, "\r") - crlf
	switch {
	case crlf > lf && crlf >= cr:
		return CRLF
	case cr > lf && cr > crlf:
		return CR
	}
	return LF
}

// NormalizeLineEndings ends every line of text with "\n", whatever it ended
// with: the editor holds text that way, and writes the file's own line
// ending back when it is saved. A lone "\r" ends a line too, even in a file
// whose lines end otherwise: Qt's text document starts a new line at one
// anyway, so it could not be kept.
func NormalizeLineEndings(text string) string {
	if !strings.Contains(text, "\r") {
		return text
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

// ApplyLineEnding ends the lines of text, which end with "\n", with l.
func ApplyLineEnding(text string, l LineEnding) string {
	if l == LF || l == "" {
		return text
	}
	return strings.ReplaceAll(text, "\n", l.Sequence())
}

// File is a file's text as the editor holds it, with the encoding and line
// ending it is written in.
type File struct {
	Text       string // with "\n" line endings
	Encoding   Encoding
	LineEnding LineEnding
}

// Parse decodes a file's bytes, guessing its encoding.
func Parse(data []byte) File {
	enc := Detect(data)
	f, err := ParseAs(data, enc)
	if err != nil { // never: Detect says UTF-8 only for valid UTF-8
		f, _ = ParseAs(data, Latin1)
	}
	return f
}

// ParseAs decodes a file's bytes in the encoding given.
func ParseAs(data []byte, enc Encoding) (File, error) {
	text, err := Decode(data, enc)
	if err != nil {
		return File{}, err
	}
	return File{Text: NormalizeLineEndings(text), Encoding: enc, LineEnding: DetectLineEnding(text)}, nil
}

// Bytes encodes the file's text in its encoding and line ending.
func (f File) Bytes() ([]byte, error) {
	data, err := Encode(ApplyLineEnding(f.Text, f.LineEnding), f.Encoding)
	if err != nil && f.LineEnding != LF {
		// Encode counts lines by "\n", which the line ending may have
		// replaced, so the text as the editor holds it says which line.
		_, err = Encode(f.Text, f.Encoding)
	}
	return data, err
}
