// errwarn warns errors and warns.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/shibukawa/configdir"
)

// Config represents config file.
type Config struct {
	// Presets is a map from string to Settings. They are lazily loaded in
	// order to "merge" settings in some case.
	Presets map[string]toml.Primitive `toml:"preset"`
}

// Setting is a set of settings which can be set with config and command line
// flags.
type Setting struct {
	// Regexp to mach errors
	ErrorFormat string
	// Regexp to mach warnings
	WarningFormat string
	// Soundset
	Soundset string
	// Read stdout of given command instead of stderr
	UseStdout bool `toml:"stdout"`
}

const (
	configFileName   string = "config.toml"
	soundsetsDirName string = "soundsets"
)

var (
	// flags specified with command line
	cmdFlags = struct {
		p, e, w, s stringFlag
		stdout     boolFlag
	}{}

	// output audio format
	format = beep.Format{
		NumChannels: 2,
		Precision:   2,
		SampleRate:  44100,
	}
)

func main() {
	parseFlags()

	setting, err := getSetting()
	exitIfErr(err)

	sounds, err := loadSounds(setting.Soundset)
	exitIfErr(err)

	err = speaker.Init(format.SampleRate, format.SampleRate.N(50*time.Millisecond))
	exitIfErr(err)

	var cmd *exec.Cmd

	switch nArgs := flag.NArg(); {
	case nArgs <= 0:
		cmd = nil

	case nArgs == 1:
		cmd = exec.Command(flag.Arg(0))

	case nArgs > 1:
		cmd = exec.Command(flag.Arg(0), flag.Args()[1:]...)
	}

	var input io.Reader

	if cmd == nil {
		input = io.TeeReader(os.Stdin, os.Stdout)
	} else if setting.UseStdout {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		stdout, err := cmd.StdoutPipe()
		exitIfErr(err)
		input = io.TeeReader(stdout, os.Stdout)
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		stderr, err := cmd.StderrPipe()
		exitIfErr(err)
		input = io.TeeReader(stderr, os.Stderr)
	}

	var matcherErr, matcherWarn *regexp.Regexp
	if setting.ErrorFormat != "" {
		matcherErr, err = regexp.Compile(setting.ErrorFormat)
		exitIfErr(err)
	}
	if setting.WarningFormat != "" {
		matcherWarn, err = regexp.Compile(setting.WarningFormat)
		exitIfErr(err)
	}

	playQueue, playerEnd := startPlayer()

	scanner := bufio.NewScanner(input)

	if cmd != nil {
		err = cmd.Start()
		exitIfErr(err)
	}

	// after here, errwarn won't exit or output anything (except for cmd's
	// output) until cmd exits.

	playQueue <- sounds.Start

	var found bool
	for scanner.Scan() {
		line := scanner.Text()

		isErr := matcherErr != nil && matcherErr.MatchString(line)
		isWarn := matcherWarn != nil && matcherWarn.MatchString(line)

		if !isErr && !isWarn {
			continue
		}

		found = true

		var newSound *beep.Buffer

		switch {
		case isErr:
			newSound = sounds.Error
		case isWarn:
			newSound = sounds.Warning
		default:
			newSound = nil
		}

		playQueue <- newSound
	}

	var exitStatus int
	var exitSound *beep.Buffer

	if cmd == nil {
		exitStatus = 0
		exitSound = sounds.Finish
	} else {
		err = cmd.Wait()

		if err == nil {
			exitStatus = 0
		} else if exitErr, ok := err.(*exec.ExitError); !ok {
			fmt.Fprintln(os.Stderr, err)
			exitStatus = 1
		} else {
			exitStatus = exitErr.ExitCode()
		}

		if exitStatus != 0 {
			exitSound = sounds.Failure
		} else if found {
			exitSound = sounds.Finish
		} else {
			exitSound = sounds.Success
		}
	}

	playQueue <- exitSound

	close(playQueue)
	<-playerEnd

	os.Exit(exitStatus)
}

// parseFlags defines and parses flags. Flags are stored in flag.CommandLine.
func parseFlags() {
	flag.Usage = func() {
		name := filepath.Base(flag.CommandLine.Name())
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", name)
		fmt.Fprintf(flag.CommandLine.Output(),
			`  %s [OPTIONS] [--] <cmdline>
  <cmdline> | %s [OPTIONS]

OPTIONS
`, name, name)
		flag.PrintDefaults()
	}

	flag.Var(&cmdFlags.p, "p", "Use `<preset>` described in config")
	flag.Var(&cmdFlags.e, "e", "Use `<regexp>` to match errors")
	flag.Var(&cmdFlags.w, "w", "Use `<regexp>` to match warnings")
	flag.Var(&cmdFlags.s, "s", "Use sounds of `<soundset>`")
	flag.Var(&cmdFlags.stdout, "stdout", "Read stdout of given cmdline instead of stderr.")
	flag.Parse()
}

// getSetting determines intended setting using flags and config file.
func getSetting() (setting Setting, err error) {
	var config Config

	cd, err := getConfigDir()
	if err != nil {
		return Setting{}, err
	}

	if !cd.Exists(configFileName) {
		if cmdFlags.p.set {
			return Setting{}, errors.New("Config file not found.")
		}
	} else {
		md, err := toml.DecodeFile(filepath.Join(cd.Path, configFileName), &config)
		if err != nil {
			return Setting{}, err
		}

		// use preset of empty name as default if it exists
		if prim, ok := config.Presets[""]; ok {
			err := md.PrimitiveDecode(prim, &setting)
			if err != nil {
				return Setting{}, err
			}
		}

		if cmd := flag.Arg(0); !cmdFlags.p.set && cmd != "" {
			cmdFlags.p.value = strings.TrimSuffix(filepath.Base(cmd), filepath.Ext(cmd))
		}

		if cmdFlags.p.value != "" {
			if prim, ok := config.Presets[cmdFlags.p.value]; !ok {
				if cmdFlags.p.set {
					return Setting{}, errors.New("Specified preset does not exist.")
				}
			} else {
				err := md.PrimitiveDecode(prim, &setting)
				if err != nil {
					return Setting{}, err
				}
			}
		}
	}

	if cmdFlags.e.set {
		setting.ErrorFormat = cmdFlags.e.value
	}
	if cmdFlags.w.set {
		setting.WarningFormat = cmdFlags.w.value
	}
	if cmdFlags.s.set {
		setting.Soundset = cmdFlags.s.value
	}
	if cmdFlags.stdout.set {
		setting.UseStdout = cmdFlags.stdout.value
	}

	if setting.Soundset != "" && !cd.Exists(filepath.Join(soundsetsDirName, setting.Soundset)) {
		return Setting{}, errors.New("Specified soundset not found.")
	}

	return setting, nil
}

// getConfigDir returns an object of configdir.Config, creating config
// directory if it does not exist.
func getConfigDir() (configdir.Config, error) {
	cds := configdir.New("", "ErrorWarner").QueryFolders(configdir.Global)

	if len(cds) <= 0 {
		return configdir.Config{}, errors.New("Unknown Error. Probably not my fault.")
	}

	cd := *cds[0]
	if !cd.Exists("") {
		err := cd.MkdirAll()
		if err != nil {
			return cd, err
		}
	}

	return cd, nil
}

// startPlayer starts a goroutine which plays sounds that are queued through
// returned send-only channel. Returned receive-only channel will be closed on
// the last sound's end after the queue channel has been closed. nil sounds
// will be ignored.
func startPlayer() (chan<- *beep.Buffer, <-chan struct{}) {
	queue := make(chan *beep.Buffer, 16)
	end := make(chan struct{})

	go func() {
		playing := make(chan struct{})
		close(playing)

		for sound := range queue {
			if sound == nil {
				continue
			}

			playing = playSound(sound)

			time.Sleep(50 * time.Millisecond)
		}

		<-playing
		close(end)
	}()

	return queue, end
}

// playSound begins playing given sound and returns a channel which closes at
// the end of the sound.
func playSound(sound *beep.Buffer) chan struct{} {
	playing := make(chan struct{})

	speaker.Clear()
	speaker.Play(beep.Seq(
		sound.Streamer(0, sound.Len()),
		beep.Callback(func() { close(playing) })))

	return playing
}

// exitIfErr will terminate errwarn with exit status 1 if error.
func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
