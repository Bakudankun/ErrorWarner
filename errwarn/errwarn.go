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

	"github.com/200sc/klangsynthese"
	"github.com/200sc/klangsynthese/audio"
	"github.com/BurntSushi/toml"
	"github.com/imdario/mergo"
	"github.com/shibukawa/configdir"
)

type Config struct {
	Options map[string]Setting `toml:"option"`
}

type Setting struct {
	ErrFormat  string `toml:"errfmt"`
	WarnFormat string `toml:"warnfmt"`
	ErrSound   string `toml:"errsound"`
	WarnSound  string `toml:"warnsound"`
	UseStdout  bool   `toml:"stdout"`
}

const (
	configFileName string = "config.toml"
)

var (
	errAudio       audio.Audio
	warnAudio      audio.Audio
	setting        Setting
	defaultSetting Setting = Setting{
		ErrFormat:  `(?i:error)`,
		WarnFormat: `(?i:warn)`,
		ErrSound:   "",
		WarnSound:  "",
		UseStdout:  false,
	}
)

func init() {
	opt := flag.String("opt", "", "Option described in config")
	errfmt := flag.String("errfmt", "", "Regexp which matches errors")
	warnfmt := flag.String("warnfmt", "", "Regexp which matches warnings")
	stdout := flag.Bool("stdout", false, "Use stdout instead of stderr")
	flag.Parse()

	option := *opt
	if option == "" {
		option = strings.TrimSuffix(filepath.Base(flag.Arg(0)), ".exe")
	}

	if err := initSetting(option); err != nil {
		if *opt == "" {
			err = initSetting("")
		}
		exitIfErr(err)
	}

	if *errfmt != "" {
		setting.ErrFormat = *errfmt
	}
	if *warnfmt != "" {
		setting.WarnFormat = *warnfmt
	}
	if *stdout {
		setting.UseStdout = *stdout
	}
}

func main() {
	var err error

	errAudio, err = klangsynthese.LoadFile(setting.ErrSound)
	exitIfErr(err)
	warnAudio, err = klangsynthese.LoadFile(setting.WarnSound)
	exitIfErr(err)

	var cmd *exec.Cmd

	switch nArgs := flag.NArg(); {
	case nArgs <= 0:
		return

	case nArgs == 1:
		cmd = exec.Command(flag.Arg(0))

	case nArgs > 1:
		cmd = exec.Command(flag.Arg(0), flag.Args()[1:]...)
	}

	var input io.ReadCloser
	var output *os.File

	if setting.UseStdout {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		input, err = cmd.StdoutPipe()
		output = os.Stdout
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		input, err = cmd.StderrPipe()
		output = os.Stderr
	}
	exitIfErr(err)

	matcherErr, err := regexp.Compile(setting.ErrFormat)
	exitIfErr(err)
	matcherWarn, err := regexp.Compile(setting.WarnFormat)
	exitIfErr(err)

	timer := time.NewTimer(0)
	scanner := bufio.NewScanner(input)

	// after this, errwarn won't exit or output anything (except for cmd's output) until cmd exits.
	cmd.Start()

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(output, line)

		isErr := matcherErr != nil && matcherErr.MatchString(line)
		isWarn := matcherWarn != nil && matcherWarn.MatchString(line)
		if !isErr && !isWarn {
			continue
		}

		if errAudio != nil {
			errAudio.Stop()
		}
		if warnAudio != nil {
			warnAudio.Stop()
		}

		var newAudio *audio.Audio

		switch {
		case isErr:
			if errAudio != nil {
				newAudio = &errAudio
			}

		case isWarn:
			if warnAudio != nil {
				newAudio = &warnAudio
			}
		}

		if newAudio == nil {
			continue
		}

		(*newAudio).Play()

		if !timer.Stop() {
			<-timer.C
		}
		timer.Reset((*newAudio).PlayLength())
		time.Sleep(50 * time.Millisecond)
	}

	<-timer.C

	err = cmd.Wait()

	var exitStatus int
	if err == nil {
		exitStatus = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitStatus = status.ExitStatus()
		}
	}

	os.Exit(exitStatus)
}

func searchAudioFile(configDir configdir.Config, basename string) (path string) {
	for _, ext := range []string{".wav", ".flac", ".mp3"} {
		if filename := basename + ext; configDir.Exists(filename) {
			return filepath.Join(configDir.Path, filename)
		}
	}

	return ""
}

func initSetting(name string) error {
	setting = defaultSetting

	configDirs := configdir.New("", "ErrorWarner").QueryFolders(configdir.Global)

	if len(configDirs) <= 0 {
		return errors.New("Unknown Error, probably not my fault.")
	}

	configDir := configDirs[0]
	if !configDir.Exists("") {
		configDir.MkdirAll()
	}

	if name != "" {
		if !configDir.Exists(configFileName) {
			return errors.New("Config file not found.")
		}

		var config Config

		if _, err := toml.DecodeFile(filepath.Join(configDir.Path, configFileName), &config); err != nil {
			return err
		}

		if option, ok := config.Options[name]; !ok {
			return errors.New("Specified option does not exist.")
		} else {
			mergo.Merge(&setting, option, mergo.WithOverride)
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

	return nil
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
