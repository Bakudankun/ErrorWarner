package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/200sc/klangsynthese"
	"github.com/200sc/klangsynthese/audio"
	"github.com/BurntSushi/toml"
	"github.com/shibukawa/configdir"
)

type Config struct {
	ErrFormat  string `toml:"errfmt"`
	WarnFormat string `toml:"warnfmt"`
}

const (
	configFileName string = "config.toml"
)

var (
	errFile       string
	warnFile      string
	errAudio      audio.Audio
	warnAudio     audio.Audio
	defaultConfig Config = Config{
		ErrFormat:  `(?i:error)`,
		WarnFormat: `(?i:warn)`,
	}
	config Config = defaultConfig
)

func init() {
	errfmt := flag.String("errfmt", "", "Regexp which matches errors")
	warnfmt := flag.String("warnfmt", "", "Regexp which matches warnings")
	flag.Parse()

	configDirs := configdir.New("", "errorwarner").QueryFolders(configdir.Global)

	if len(configDirs) <= 0 {
		return
	}

	configDir := configDirs[0]
	if !configDir.Exists("") {
		configDir.MkdirAll()
		return
	}

	errFile = searchAudioFile(*configDir, "error")
	warnFile = searchAudioFile(*configDir, "warn")

	if configDir.Exists(configFileName) {
		toml.DecodeFile(filepath.Join(configDir.Path, configFileName), &config)
	}

	if *errfmt != "" {
		config.ErrFormat = *errfmt
	}
	if *warnfmt != "" {
		config.WarnFormat = *warnfmt
	}
}

func main() {
	errAudio, _ = klangsynthese.LoadFile(errFile)
	warnAudio, _ = klangsynthese.LoadFile(warnFile)

	var cmd *exec.Cmd

	switch nArgs := flag.NArg(); {
	case nArgs <= 0:
		return

	case nArgs == 1:
		cmd = exec.Command(flag.Arg(0))

	case nArgs > 1:
		cmd = exec.Command(flag.Arg(0), flag.Args()[1:]...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	stderr, _ := cmd.StderrPipe()

	matcherErr, _ := regexp.Compile(config.ErrFormat)
	matcherWarn, _ := regexp.Compile(config.WarnFormat)

	timer := time.NewTimer(0)
	scanner := bufio.NewScanner(stderr)

	cmd.Start()

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(os.Stderr, line)

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
}

func searchAudioFile(configDir configdir.Config, basename string) (path string) {
	for _, ext := range []string{".wav", ".flac", ".mp3"} {
		filename := basename + ext
		if configDir.Exists(filename) {
			return filepath.Join(configDir.Path, filename)
		}
	}

	return ""
}
