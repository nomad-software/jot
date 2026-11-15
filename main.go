// Notepad is a simple text editor for Linux, with tabs, in the style of
// gedit.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"

	"notepad/internal/appicon"
	"notepad/internal/config"
	"notepad/internal/desktop"
	"notepad/internal/ui"
)

func main() {
	cfgPath := flag.String("config", config.Path(), "settings file")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile to this file")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: notepad [flags] [file...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("notepad", ui.AppVersion())
		return
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	cfg, cfgErr := config.Load(*cfgPath)
	if errors.Is(cfgErr, os.ErrPermission) {
		fatal(cfgErr)
	}

	ui.NewApplication()
	qtcore.QCoreApplicationSetApplicationName("notepad")
	// No application display name is set: Qt would append it to every
	// window title, which has the editor's name already.

	// Under Wayland this is the name a window goes by, which a desktop
	// matches to the desktop entry of that name for its icon and its place
	// in the task bar.
	qtgui.QGuiApplicationSetDesktopFileName(desktop.Name)
	qtgui.QGuiApplicationSetWindowIcon(ui.AppIcon())

	// The desktop is given an entry for the program, which starts the copy
	// that is running now, wherever it is, so that it can be started from
	// the desktop's menus, kept in a dock, and chosen to open text files.
	if os.Getenv("NOTEPAD_NO_DESKTOP_ENTRY") == "" {
		if err := desktop.Install(appicon.PNG); err != nil {
			fmt.Fprintln(os.Stderr, "notepad: the desktop entry could not be written:", err)
		}
	}

	code := qute.Main(func() {
		ui.NewWindow(cfg, *cfgPath, cfgErr, flag.Args())
	})
	if code != 0 {
		pprof.StopCPUProfile() // os.Exit skips the deferred stop
		os.Exit(code)
	}
}

func fatal(err error) {
	pprof.StopCPUProfile() // os.Exit skips the deferred stop; a no-op unless profiling
	fmt.Fprintln(os.Stderr, "notepad:", err)
	os.Exit(1)
}
