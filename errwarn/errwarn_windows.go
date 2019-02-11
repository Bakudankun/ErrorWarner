package main

import (
	"bufio"
	"fmt"
	"github.com/200sc/klangsynthese"
	"github.com/200sc/klangsynthese/audio"
	"github.com/shibukawa/configdir"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

var (
	errFile   string
	warnFile  string
	errAudio  audio.Audio
	warnAudio audio.Audio
)

func init() {
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
}

func main() {
	errAudio, _ = klangsynthese.LoadFile(errFile)
	warnAudio, _ = klangsynthese.LoadFile(warnFile)

	var cmd *exec.Cmd

	switch nArgs := len(os.Args) - 1; {
	case nArgs <= 0:
		return

	case nArgs == 1:
		cmd = exec.Command(os.Args[1])

	case nArgs > 1:
		cmd = exec.Command(os.Args[1], os.Args[2:]...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	stderr, _ := cmd.StderrPipe()

	timer := time.NewTimer(0)
	scanner := bufio.NewScanner(stderr)

	cmd.Start()

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(os.Stderr, line)

		matcherErr := regexp.MustCompile(`(?i:error)`)
		matcherWarn := regexp.MustCompile(`(?i:warn)`)
		if !matcherErr.MatchString(line) && !matcherWarn.MatchString(line) {
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
		case matcherErr.MatchString(line):
			if errAudio != nil {
				newAudio = &errAudio
			}

		case matcherWarn.MatchString(line):
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
