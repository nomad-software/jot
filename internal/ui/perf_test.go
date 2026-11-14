package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qutetest"
)

// TestLargeFileTimes reports how long a large file takes to open, type in,
// save and search. It is run by hand: JOT_PERF=1 go test -run Large -v.
func TestLargeFileTimes(t *testing.T) {
	if os.Getenv("JOT_PERF") == "" {
		t.Skip("set JOT_PERF=1 to time a large file")
	}
	var b strings.Builder
	for i := range 200000 {
		fmt.Fprintf(&b, "%06d\tThe quick brown fox jumps over the lazy dog, again and again.\n", i)
	}
	path := writeFile(t, "big.txt", []byte(b.String()))
	qutetest.Run(t, func() {
		w := newTestWindow(t, nil)
		start := time.Now()
		w.openFiles([]string{path})
		qutetest.Wait(1)
		t.Logf("open %d MB, %d lines: %v", b.Len()>>20, w.current().edit.BlockCount(), time.Since(start))

		d := w.current()
		start = time.Now()
		for range 200 {
			qutetest.Key(d.edit, qtcore.Key_A, qtcore.NoModifier)
		}
		t.Logf("200 keys at the start: %v each", time.Since(start)/200)

		start = time.Now()
		w.run("save", false)
		t.Logf("save: %v", time.Since(start))

		w.run("find", false)
		start = time.Now()
		qutetest.Type(w.find.find, "fox")
		t.Logf("find and highlight %s: %v", w.find.count.Text(), time.Since(start))
	})
}
