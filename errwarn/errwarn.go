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
	ErrFormat  string `toml:"errfmt"`
	WarnFormat string `toml:"warnfmt"`
	ErrSound   string `toml:"errsound"`
	WarnSound  string `toml:"warnsound"`
	UseStdout  bool   `toml:"stdout"`
}

const (
	configFileName string          = "config.toml"
	sampleRate     beep.SampleRate = 44100
)

var (
	errSound       *beep.Buffer
	warnSound      *beep.Buffer
	setting        Setting
	defaultSetting = Setting{
		ErrFormat:  "",
		WarnFormat: "",
		ErrSound:   "",
		WarnSound:  "",
		UseStdout:  false,
	}
	format = beep.Format{
		NumChannels: 2,
		Precision:   2,
		SampleRate:  sampleRate,
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

	var p, e, w stringFlag
	var stdout boolFlag
	flag.Var(&p, "p", "Use `<preset>` described in config")
	flag.Var(&e, "e", "Use `<regexp>` to match errors")
	flag.Var(&w, "w", "Use `<regexp>` to match warnings")
	flag.Var(&stdout, "stdout", "Read stdout of cmd instead of stderr")
	flag.Parse()

	err := initSetting(p, e, w, stdout)
	exitIfErr(err)
}

func main() {
	var err error

	if setting.ErrSound != "" {
		errSound, err = loadAudioFile(setting.ErrSound)
		exitIfErr(err)
	}
	if setting.WarnSound != "" {
		warnSound, err = loadAudioFile(setting.WarnSound)
		exitIfErr(err)
	}

	err = speaker.Init(sampleRate, sampleRate.N(50*time.Millisecond))
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
	if setting.ErrFormat != "" {
		matcherErr, err = regexp.Compile(setting.ErrFormat)
		exitIfErr(err)
	}
	if setting.WarnFormat != "" {
		matcherWarn, err = regexp.Compile(setting.WarnFormat)
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

	for scanner.Scan() {
		line := scanner.Text()

		isErr := matcherErr != nil && matcherErr.MatchString(line)
		isWarn := matcherWarn != nil && matcherWarn.MatchString(line)

		var newSound *beep.Buffer

		switch {
		case isErr:
			newSound = errSound
		case isWarn:
			newSound = warnSound
		default:
			newSound = nil
		}

		if newSound == nil {
			continue
		}

		speaker.Clear()

		playing = make(chan struct{})
		speaker.Play(beep.Seq(
			newSound.Streamer(0, newSound.Len()),
			beep.Callback(func() { close(playing) })))

		time.Sleep(50 * time.Millisecond)
	}

	<-playing

	if cmd == nil {
		os.Exit(0)
	}

	err = cmd.Wait()

	var exitStatus int
	if err == nil {
		exitStatus = 0
	} else if exitErr, ok := err.(*exec.ExitError); !ok {
		exitIfErr(err)
	} else if status, ok := exitErr.Sys().(syscall.WaitStatus); !ok {
		exitIfErr(err)
	} else {
		exitStatus = status.ExitStatus()
	}

	os.Exit(exitStatus)
}

func getConfigDir() *configdir.Config {
	configDirs := configdir.New("", "ErrorWarner").QueryFolders(configdir.Global)

	if len(configDirs) <= 0 {
		return nil
	}

	configDir := configDirs[0]
	if !configDir.Exists("") {
		configDir.MkdirAll()
	}

	return configDir
}

func initSetting(p, e, w stringFlag, stdout boolFlag) error {
	setting = defaultSetting

	configDir := getConfigDir()
	if configDir == nil {
		return errors.New("Unknown Error. Probably not my fault.")
	}

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

	if setting.ErrSound == "" {
		setting.ErrSound = searchAudioFile(*configDir, "error")
	} else if !filepath.IsAbs(setting.ErrSound) {
		setting.ErrSound = filepath.Join(configDir.Path, setting.ErrSound)
	}

	if setting.WarnSound == "" {
		setting.WarnSound = searchAudioFile(*configDir, "warn")
	} else if !filepath.IsAbs(setting.WarnSound) {
		setting.WarnSound = filepath.Join(configDir.Path, setting.WarnSound)
	}

	if e.set {
		setting.ErrFormat = e.value
	}
	if w.set {
		setting.WarnFormat = w.value
	}
	if stdout.set {
		setting.UseStdout = stdout.value
	}

	return nil
}

func searchAudioFile(configDir configdir.Config, basename string) (path string) {
	for _, ext := range []string{".wav", ".flac", ".mp3", ".ogg"} {
		if filename := basename + ext; configDir.Exists(filename) {
			return filepath.Join(configDir.Path, filename)
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
	buffer.Append(beep.Resample(3, f.SampleRate, sampleRate, s))

	s.Close()

	return buffer, nil
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
