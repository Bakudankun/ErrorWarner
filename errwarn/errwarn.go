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
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
	"github.com/shibukawa/configdir"
)

type Config struct {
	Presets map[string]toml.Primitive `toml:"preset"`
}

type Setting struct {
	ErrorFormat   string
	WarningFormat string
	Soundset      string
	UseStdout     bool `toml:"stdout"`
}

type soundset struct {
	Error   *beep.Buffer `file:"error"`
	Warning *beep.Buffer `file:"warn"`
	Finish  *beep.Buffer `file:"finish"`
	Success *beep.Buffer `file:"success"`
	Failure *beep.Buffer `file:"fail"`
}

func (s *soundset) load() error {
	if s == nil {
		return errors.New("Internal error.")
	}

	sv := reflect.ValueOf(s).Elem()
	st := reflect.TypeOf(*s)

	for i := 0; i < st.NumField(); i++ {
		name := st.Field(i).Tag.Get("file")

		path := searchAudioFile(name)
		if path == "" {
			continue
		}

		b, err := loadAudioFile(path)
		if err != nil {
			return err
		}

		sv.Field(i).Set(reflect.ValueOf(b))
	}

	return nil
}

const (
	configFileName   string = "config.toml"
	soundsetsDirName string = "soundsets"
)

var (
	configDir configdir.Config
	setting   Setting
	format    = beep.Format{
		NumChannels: 2,
		Precision:   2,
		SampleRate:  44100,
	}
)

func init() {
	flag.Usage = func() {
		name := flag.CommandLine.Name()
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", name)
		fmt.Fprintf(flag.CommandLine.Output(),
			`  %s [-p <preset>] [-e <regexp>] [-w <regexp>] [-stdout[=true|false]] [--] <cmd>
  <cmd> | %s [-p <preset>] [-e <regexp>] [-w <regexp>]

`, name, name)
		flag.PrintDefaults()
	}

	var err error
	configDir, err = getConfigDir()
	exitIfErr(err)

	var p, e, w, s stringFlag
	var stdout boolFlag
	flag.Var(&p, "p", "Use `<preset>` described in config")
	flag.Var(&e, "e", "Use `<regexp>` to match errors")
	flag.Var(&w, "w", "Use `<regexp>` to match warnings")
	flag.Var(&s, "s", "Use sounds of `<soundset>`")
	flag.Var(&stdout, "stdout", "Read stdout of cmd instead of stderr")
	flag.Parse()

	err = initSetting(p, e, w, s, stdout)
	exitIfErr(err)
}

func main() {
	var err error
	var sounds soundset

	err = sounds.load()
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
		err = nil
	} else if setting.UseStdout {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		var stdout io.Reader
		stdout, err = cmd.StdoutPipe()
		input = io.TeeReader(stdout, os.Stdout)
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		var stderr io.Reader
		stderr, err = cmd.StderrPipe()
		input = io.TeeReader(stderr, os.Stderr)
	}
	exitIfErr(err)

	var matcherErr, matcherWarn *regexp.Regexp
	if setting.ErrorFormat != "" {
		matcherErr, err = regexp.Compile(setting.ErrorFormat)
		exitIfErr(err)
	}
	if setting.WarningFormat != "" {
		matcherWarn, err = regexp.Compile(setting.WarningFormat)
		exitIfErr(err)
	}

	playing := make(chan struct{})
	close(playing)
	scanner := bufio.NewScanner(input)

	if cmd != nil {
		err = cmd.Start()
		exitIfErr(err)
	}

	// after here, errwarn won't exit or output anything (except for cmd's output) until cmd exits.

	var found bool
	for scanner.Scan() {
		line := scanner.Text()

		isErr := matcherErr != nil && matcherErr.MatchString(line)
		isWarn := matcherWarn != nil && matcherWarn.MatchString(line)

		var newSound *beep.Buffer

		switch {
		case isErr:
			newSound = sounds.Error
		case isWarn:
			newSound = sounds.Warning
		default:
			newSound = nil
		}

		if newSound == nil {
			continue
		}

		found = true
		speaker.Clear()

		playing = make(chan struct{})
		speaker.Play(beep.Seq(
			newSound.Streamer(0, newSound.Len()),
			beep.Callback(func() { close(playing) })))

		time.Sleep(50 * time.Millisecond)
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
		} else if status, ok := exitErr.Sys().(syscall.WaitStatus); !ok {
			fmt.Fprintln(os.Stderr, err)
			exitStatus = 1
		} else {
			exitStatus = status.ExitStatus()
		}

		if exitStatus != 0 {
			exitSound = sounds.Failure
		} else if found {
			exitSound = sounds.Finish
		} else {
			exitSound = sounds.Success
		}
	}

	if exitSound != nil {
		speaker.Clear()

		playing = make(chan struct{})
		speaker.Play(beep.Seq(
			exitSound.Streamer(0, exitSound.Len()),
			beep.Callback(func() { close(playing) })))
	}

	<-playing

	os.Exit(exitStatus)
}

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

func initSetting(p, e, w, s stringFlag, stdout boolFlag) error {
	var config Config

	if !configDir.Exists(configFileName) {
		if p.set {
			return errors.New("Config file not found.")
		}
	} else {
		md, err := toml.DecodeFile(filepath.Join(configDir.Path, configFileName), &config)
		if err != nil {
			return err
		}

		// use preset of empty name as default if it exists
		if prim, ok := config.Presets[""]; ok {
			err := md.PrimitiveDecode(prim, &setting)
			if err != nil {
				return err
			}
		}

		if cmd := flag.Arg(0); !p.set && cmd != "" {
			p.value = strings.TrimSuffix(filepath.Base(cmd), filepath.Ext(cmd))
		}

		if p.value != "" {
			if prim, ok := config.Presets[p.value]; !ok {
				if p.set {
					return errors.New("Specified preset does not exist.")
				}
			} else {
				err := md.PrimitiveDecode(prim, &setting)
				if err != nil {
					return err
				}
			}
		}
	}

	if e.set {
		setting.ErrorFormat = e.value
	}
	if w.set {
		setting.WarningFormat = w.value
	}
	if s.set {
		setting.Soundset = s.value
	}
	if stdout.set {
		setting.UseStdout = stdout.value
	}

	if setting.Soundset != "" && !configDir.Exists(filepath.Join(soundsetsDirName, setting.Soundset)) {
		return errors.New("Specified soundset not found.")
	}

	return nil
}

func searchAudioFile(name string) (path string) {
	var dir string

	if setting.Soundset != "" {
		dir = filepath.Join(soundsetsDirName, setting.Soundset)
	}

	for _, ext := range []string{".wav", ".flac", ".mp3", ".ogg"} {
		if relpath := filepath.Join(dir, name+ext); configDir.Exists(relpath) {
			return filepath.Join(configDir.Path, relpath)
		}
	}

	return ""
}

func loadAudioFile(path string) (*beep.Buffer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var s beep.StreamCloser
	var f beep.Format

	switch filepath.Ext(path) {
	case ".wav":
		s, f, err = wav.Decode(file)
	case ".mp3":
		s, f, err = mp3.Decode(file)
	case ".flac":
		s, f, err = flac.Decode(file)
	case ".ogg":
		s, f, err = vorbis.Decode(file)
	}

	if err != nil {
		return nil, err
	}

	buffer := beep.NewBuffer(format)
	buffer.Append(beep.Resample(4, f.SampleRate, format.SampleRate, s))

	s.Close()

	return buffer, nil
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
