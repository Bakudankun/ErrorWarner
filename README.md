[にほんご](https://qiita.com/Bakudankun/items/01d74683b9716dcf45bf)

# ErrorWarner

ErrorWarner plays sound when errors or warnings are found while build process.

[DEMO](https://twitter.com/Bakudankun/status/1099633393468268544)


## Installation

### Platforms

* Windows
* macOS
* Linux
* other OSs which [Oto] supports (hopefully)


### Build

Install [Go] and run following command.

```
go get -v github.com/Bakudankun/ErrorWarner/errwarn
```

Library package may be required on Linux. See README of [Oto].


### Place sound files

Currently no sound data is bundled. Please prepare ones by yourself.

```
errwarn ""
```

This command line should fail, and creates `ErrorWarner` directory in following
directory.

* Windows: `%APPDATA%` (usually `C:\Users\USERNAME\AppData\Roaming`)
* macOS: `$HOME/Library/Application Support`
* Linux: `$HOME/.config`

Place sound files in it and name it `error.*` or `warn.*`. Supported formats
are WAV, MP3, FLAC and Ogg Vorbis.


## Usage

```
$ errwarn -h
Usage of errwarn:
  errwarn [OPTIONS] [--] <cmdline>
  <cmdline> | errwarn [OPTIONS]

OPTIONS
  -e <regexp>
        Use <regexp> to match errors
  -p <preset>
        Use <preset> described in config
  -s <soundset>
        Use sounds of <soundset>
  -stdout
        Read stdout of given <cmdline> instead of stderr.
  -w <regexp>
        Use <regexp> to match warnings
```


### Pass a command line to ErrorWarner

```
errwarn [OPTIONS] [--] <cmdline>
```

`errwarn` executes given `<cmdline>` and reads its output. When a line matches
the [regular expressions][Go-Regexp] specified with `-e` or `-w`, ErrorWarner
notify you with sound. The exit status will be the same with that of
`<cmdline>` unless `errwarn` itself throws an error.

`errwarn` reads standard error by default. Set `-stdout` if `<cmdline>` writes
logs to standard output.


### Read standard input

```
<cmdline> | errwarn [OPTIONS]
```

If `<cmdline>` is not given as argument, `errwarn` reads its standard input.
Note that `errwarn` likely to exit successfully regardless of exit status of
`<cmdline>`. `-stdout` option is ignored in this form.


## Presets

To reuse regular expressions, you can create presets to switch with `-p` option
by writing `config.toml` file in ErrorWarner directory created
[above](#place-sound-files).

Example of config.toml:

```toml
# preset with empty name is used to set default values
[preset.""]
stdout = true               # read stdout instead of stderr
errorFormat = '(?i:error)'  # regexp to mach errors
warningFormat = '(?i:warn)' # regexp to mach warnings
soundset = 'foo'            # soundset

# call this preset like `errwarn -p gcc`
[preset.gcc]
errorFormat = '^\S+:\d+:\d+: error: '
warningFormat = '^\S+:\d+:\d+: warning: '
soundset = 'bar'

# If preset name matches the executing command's name (extension trimmed), the
# preset is selected automatically.
[preset.go]
stdout = false
errorFormat = '^.*: '
warningFormat = '' # disabled if empty
```


## Soundsets

You can create soundsets by creating `soundsets/*` directory under the
ErrorWarner directory.

```
ErrorWarner
|-- config.toml
|-- error.wav
|-- warn.wav
|-- ...
|
+-- soundsets
    +-- foo
    |   |-- error.ogg
    |   |-- warn.ogg
    |   |-- ...
    |
    +-- bar
        |-- error.flac
        |-- warn.flac
        |-- ...
```

Soundsets can be specified with `-s` option or `soundset` param in config. If
none or empty name is specified, sound files right under ErrorWarner directory
will be used.

Sound files of following names are used.

* `error.*`  
  Played when a error is found.
* `warn.*`  
  Played when a warnings is found.
* `start.*`  
  Played on given command's starting.
* `finish.*`  
  Played if given command exited successfully though some errors or warnings
  found. If errwarn is reading its stdin, this always be played on exit.
* `success.*`  
  Played if given command exited successfully with no errors or warnings. Not
  used if errwarn is reading its stdin.
* `fail.*`  
  Played if given command exited unsuccessfully. Not used if errwarn is reading
  its stdin.


## License

[MIT](https://github.com/Bakudankun/ErrorWarner/blob/master/LICENSE)


Happy Erroring!!


[Oto]: https://github.com/hajimehoshi/oto
[Go]: https://golang.org/
[Go-Regexp]: https://golang.org/pkg/regexp/syntax/

