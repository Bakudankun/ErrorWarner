# ErrorWarner

ビルド中にerrorやwarningが出てきたときに音で賑やかします。


## じゅんぼ

### 対応OS

* Windows
* macOS
* Linux
* その他[Oto]がサポートしてるOS（たぶん）


### インストール

[Go]をいい感じにインストールし、以下のコマンドを実行します。

```
go get -v github.com/Bakudankun/ErrorWarner/errwarn
```

LinuxやFreeBSDの場合はライブラリのインストールが必要になるかもしれません。
詳しくは[Oto]のREADMEで。


### サウンドファイルの配置

今のところサウンドはついてこないのでご自分で用意してください。
適当に`TTS download`とかでググって出てきたサイトで読み上げさせたのを使うと良いと思います。

サウンドを用意したら以下のコマンドを実行します。

```
echo hoge | errwarn
```

すると、Windowsなら`%APPDATA%`の中、macOSなら`$HOME/Library/Application Support`の中、
Linuxなら`$HOME/.config`の中に`ErrorWarner`フォルダを作成しながら`hoge`と出力されるので、
そのフォルダの中に`error.wav`と`warn.wav`を置いてください。

ファイル形式はWAV・MP3・FLAC・Ogg Vorbisに対応してるといいな。


## 使い方

### おためし

```
errwarn -e error -w warn
```

と実行すると標準入力を待機するので、試しに`error`または`warn`と書いてエンターを押してください。
ちゃんと音声ファイルが設置されていればそれが再生されるハズです。

気が済んだらCtrl-Cで終了します。


### ErrorWarnerにコマンドを渡す

```
errwarn [-p <preset>] [-e <regexp>] [-w <regexp>] [-stdout] [--] <cmd>
```

ErrorWarnerが`<cmd>`を実行し、その標準エラー出力を読みます。

出力の中に`-e`や`-w`で指定した正規表現にマッチする行が出てきたらサウンドを鳴らしてお知らせします。

終了ステータスは`<cmd>`の終了ステータスを返します。

ビルドログが標準出力に出力されるコンパイラの場合は`-sdtout`を指定してください。


### ErrorWarnerにパイプする

```
<cmd> | errwarn [-p <preset>] [-e <regexp>] [-w <regexp>]
```

最初に`errwarn`を書き忘れた場合はパイプさせることもできます。

ただし標準エラー出力はパイプされないので、ビルドログが標準エラー出力に出力される場合は
どうにかして標準出力に流し込む必要があります。

また、この場合`errwarn`は`cmd`の終了ステータスにかかわらず正常終了することにも注意。


### オプション

* `-e <regexp>`

  errorの行にマッチする[正規表現][Go-regexp]を指定します。

* `-w <regexp>`

  warningの行にマッチする[正規表現][Go-regexp]を指定します。

* `-stdout`

  標準エラー出力の代わりに標準出力を読みます。

* `-p <preset>`

  設定ファイルに書いたプリセットを指定します。


## プリセット

一々正規表現を書くのが面倒な場合は、上で作成されたErrorWarnerフォルダに`config.toml`ファイルを置いておくことで、`-p`オプションでプリセットを呼び出せるようになります。

`config.toml`ファイルではプリセットごとにサウンドを指定することもできます。

例：

```toml:config.toml
# 空文字列のプリセットを作るとデフォルト値として使われる
[preset.""]
stdout = true                        # 標準エラー出力の代わりに標準出力を読む
errfmt = '(?i:error)'                # errorの行にマッチする正規表現
warnfmt = '(?i:warn)'                # warningの行にマッチする正規表現
errsound = 'sound/default/error.wav' # errorを見つけたときに流すサウンド
warnsound = 'sound/default/warn.wav' # warningを見つけたときに流すサウンド

# `errwarn -p myvoice` でこの設定を呼び出せるようになる
[preset.myvoice]
# サウンドファイルパスを相対パスにした場合はErrorWarnerフォルダからの相対パス
errsound = 'sound/my/error.wav'
warnsound = 'sound/my/warn.wav'

# プリセットの名前をコマンドの名前（拡張子無し）にすると、
# コマンド渡しで実行したときに自動で選択してくれる
[preset.go]
stdout = false
errfmt = '^.*: '
warnfmt = 'a^' # 何にもマッチさせない
```


Happy Erroring!!


[Oto]: https://github.com/hajimehoshi/oto
[Go]: https://golang.org/
[Go-regexp]: https://golang.org/pkg/regexp/syntax/

