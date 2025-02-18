package main

import (
	"os"
	"path/filepath"
	"reflect"

	"golang.org/x/sync/errgroup"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// soundset holds sound data in memory.
type soundset struct {
	Error   *beep.Buffer `file:"error"`
	Warning *beep.Buffer `file:"warn"`
	Start   *beep.Buffer `file:"start"`
	Finish  *beep.Buffer `file:"finish"`
	Success *beep.Buffer `file:"success"`
	Failure *beep.Buffer `file:"fail"`
}

// loadSounds loads sound files in soundset directory of ssName. If ssName is
// empty, loadSounds loads sound files right under the config directory.
func loadSounds(ssName string) (s soundset, err error) {
	// Iterate for members of soundset
	sv := reflect.ValueOf(&s).Elem()
	st := reflect.TypeOf(s)

	var eg errgroup.Group
	for i := 0; i < st.NumField(); i++ {
		i := i
		eg.Go(func() error {
			name := st.Field(i).Tag.Get("file")

			path := searchAudioFile(ssName, name)
			if path == "" {
				return nil
			}

			b, err := loadAudioFile(path)
			if err != nil {
				return err
			}

			sv.Field(i).Set(reflect.ValueOf(b))
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return soundset{}, err
	}

	return s, nil
}

// searchAudioFile searches a sound file named name in soundset directory of
// ssName. name should not include extension.
// `*.wav`, `*.flac`, `*.mp3` and `*.ogg` files are searched in this order.
// If ssName is empty, searchAudioFile searches right under the config
// directory.
func searchAudioFile(ssName, name string) (path string) {
	var dir string
	cd, _ := getConfigDir()

	if ssName != "" {
		dir = filepath.Join(soundsetsDirName, ssName)
	}

	for _, ext := range []string{".wav", ".flac", ".mp3", ".ogg"} {
		if relpath := filepath.Join(dir, name+ext); cd.Exists(relpath) {
			return filepath.Join(cd.Path, relpath)
		}
	}

	return ""
}

// loadAudioFile loads a sound file of path and return pointer to its buffer.
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
