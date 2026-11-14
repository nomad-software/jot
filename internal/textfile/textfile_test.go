package textfile

import (
	"bytes"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want Encoding
	}{
		{"empty", nil, UTF8},
		{"ascii", []byte("hello\n"), UTF8},
		{"utf8", []byte("héllo wörld ✓\n"), UTF8},
		{"utf8 bom", append([]byte{0xef, 0xbb, 0xbf}, "hi"...), UTF8BOM},
		{"utf16le bom", []byte{0xff, 0xfe, 'h', 0, 'i', 0}, UTF16LE},
		{"utf16be bom", []byte{0xfe, 0xff, 0, 'h', 0, 'i'}, UTF16BE},
		{"utf32le bom", []byte{0xff, 0xfe, 0, 0, 'h', 0, 0, 0}, UTF32LE},
		{"utf32be bom", []byte{0, 0, 0xfe, 0xff, 0, 0, 0, 'h'}, UTF32BE},
		{"utf16le no bom", []byte("h\x00e\x00l\x00l\x00o\x00\n\x00"), UTF16LE},
		{"utf16be no bom", []byte("\x00h\x00e\x00l\x00l\x00o\x00\n"), UTF16BE},
		{"latin1", []byte("caf\xe9\n"), Latin1},
	}
	for _, tt := range tests {
		if got := Detect(tt.data); got != tt.want {
			t.Errorf("%s: Detect = %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	text := "plain ascii\nLatin: café ÿ\nGreek: λόγος\nEmoji: 😀\nCJK: 漢字\n"
	for _, enc := range Encodings {
		in := text
		if enc == Latin1 {
			in = "plain ascii\nLatin: café ÿ\n"
		}
		data, err := Encode(in, enc)
		if err != nil {
			t.Fatalf("%s: Encode: %v", enc, err)
		}
		if got := Detect(data); got != enc {
			t.Errorf("%s: Detect of encoded text = %s", enc, got)
		}
		out, err := Decode(data, enc)
		if err != nil {
			t.Fatalf("%s: Decode: %v", enc, err)
		}
		if out != in {
			t.Errorf("%s: round trip = %q, want %q", enc, out, in)
		}
	}
}

func TestEncodeLatin1Unrepresentable(t *testing.T) {
	_, err := Encode("ok\nnot ok: λ", Latin1)
	if err == nil || !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "U+03BB") {
		t.Fatalf("err = %v, want one naming line 2 and U+03BB", err)
	}
}

func TestDecodeInvalidUTF8(t *testing.T) {
	if _, err := Decode([]byte("caf\xe9"), UTF8); err == nil {
		t.Fatal("invalid UTF-8 decoded without an error")
	}
}

func TestDecodeShortUnits(t *testing.T) {
	if got, _ := Decode([]byte{'a', 0, 'b'}, UTF16LE); got != "a�" {
		t.Errorf("UTF-16 with an odd byte = %q", got)
	}
	if got, _ := Decode([]byte{'a', 0, 0, 0, 'b'}, UTF32LE); got != "a�" {
		t.Errorf("UTF-32 with a short unit = %q", got)
	}
	if got, _ := Decode([]byte{0xff, 0xff, 0x11, 0}, UTF32LE); got != "�" {
		t.Errorf("UTF-32 beyond Unicode = %q", got)
	}
}

func TestLineEndings(t *testing.T) {
	tests := []struct {
		in   string
		want LineEnding
	}{
		{"", LF},
		{"one line", LF},
		{"a\nb\n", LF},
		{"a\r\nb\r\n", CRLF},
		{"a\rb\r", CR},
		{"a\r\nb\r\nc\n", CRLF},
		{"a\nb\nc\r\n", LF},
	}
	for _, tt := range tests {
		if got := DetectLineEnding(tt.in); got != tt.want {
			t.Errorf("DetectLineEnding(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
	if got := NormalizeLineEndings("a\r\nb\rc\nd"); got != "a\nb\nc\nd" {
		t.Errorf("NormalizeLineEndings = %q", got)
	}
	for _, le := range LineEndings {
		if got := NormalizeLineEndings(ApplyLineEnding("a\nb\n", le)); got != "a\nb\n" {
			t.Errorf("%s: apply then normalize = %q", le, got)
		}
	}
}

func TestParseAndBytes(t *testing.T) {
	data := []byte("\xff\xfeo\x00n\x00e\x00\r\x00\n\x00t\x00w\x00o\x00\r\x00\n\x00")
	f := Parse(data)
	if f.Encoding != UTF16LE || f.LineEnding != CRLF || f.Text != "one\ntwo\n" {
		t.Fatalf("Parse = %+v", f)
	}
	out, err := f.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Errorf("Bytes = %q, want %q", out, data)
	}
}

func TestLooksBinary(t *testing.T) {
	if LooksBinary([]byte("text\n"), UTF8) {
		t.Error("text looks binary")
	}
	if !LooksBinary([]byte("ELF\x00\x01"), UTF8) {
		t.Error("NUL bytes do not look binary")
	}
	utf16, _ := Encode("text\n", UTF16LE)
	if LooksBinary(utf16, UTF16LE) {
		t.Error("UTF-16 text looks binary")
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add("hello\r\nworld")
	f.Add("λ😀\r")
	f.Fuzz(func(t *testing.T, s string) {
		if !strings.ContainsRune(s, '�') && utf8Valid(s) {
			for _, enc := range []Encoding{UTF8, UTF8BOM, UTF16LE, UTF16BE, UTF32LE, UTF32BE} {
				data, err := Encode(s, enc)
				if err != nil {
					t.Fatal(err)
				}
				got, err := Decode(data, enc)
				if err != nil || got != s {
					t.Fatalf("%s: %q -> %q, %v", enc, s, got, err)
				}
			}
		}
		_ = Parse([]byte(s)) // never panics
	})
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "\x00") == s }

// TestLatin1ErrorLine checks that a character Latin-1 cannot hold is said to
// be on its line whatever the line ending.
func TestLatin1ErrorLine(t *testing.T) {
	for _, le := range LineEndings {
		f := File{Text: "a\nb\n\u20ac\n", Encoding: Latin1, LineEnding: le}
		if _, err := f.Bytes(); err == nil || !strings.Contains(err.Error(), "line 3") {
			t.Errorf("%s: %v, want an error naming line 3", le, err)
		}
	}
}
