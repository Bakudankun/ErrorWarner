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

お好みで`ew`などを`errwarn`へのエイリアスに設定しておきましょ。

Linuxとかはライブラリのインストールが必要になるかもしれません。詳しくは[Oto]のREADMEで。


### サウンドファイルの配置

今のところサウンドはついてこないのでご自分で用意してください。
適当に`TTS download`とかでググって出てきたサイトで読み上げさせたのを使うと良いと思います。

サウンドを用意したら以下のコマンドを実行します。

```
errwarn ""
```

このコマンドは失敗しますが、Windowsなら`%APPDATA%`の中、macOSなら`$HOME/Library/Application Support`の中、
Linuxなら`$HOME/.config`の中に`ErrorWarner`フォルダが作成されます。
そのフォルダの中に`error.wav`や`warn.wav`を置いたり置かなかったりしてください。

ファイル形式はWAV・MP3・FLAC・Ogg Vorbisに対応してるといいな。


## つかう

### おためし

```
errwarn -e error -w warn
```

と実行すると標準入力を待機するので、試しに`error`または`warn`と書いてエンターを押してください。
ちゃんと音声ファイルが設置されていればそれが再生されるハズです。

気が済んだらCtrl-Cで終了します。


### ErrorWarnerにコマンドを渡す

```
errwarn [-p <preset>] [-e <regexp>] [-w <regexp>] [-s <soundset>] [-stdout[=true|false]] [--] <cmdline>
```

ErrorWarnerが`<cmdline>`を実行し、その出力を読みます。

出力の中に`-e`や`-w`で指定した[正規表現]にマッチする行が出てきたらサウンドを鳴らしてお知らせします。

`errwarn`自身が異常終了しない限り、終了ステータスは`<cmdline>`の終了ステータスを返します。

デフォルトでは標準エラー出力を読みます。ビルドログが標準出力に出てくる場合は`-stdout`を指定してください。


### ErrorWarnerにパイプする

```
<cmdline> | errwarn [-p <preset>] [-e <regexp>] [-w <regexp>] [-s <soundset>]
```

最初に`errwarn`を書き忘れた場合はパイプさせることもできます。

ただし標準エラー出力はパイプされないので、ビルドログが標準エラー出力に出てくる場合はどうにかしてパイプに流し込む必要があります。

また、この場合`errwarn`は`cmdline`の終了ステータスにかかわらず基本的に正常終了することにも注意。


### オプション

* `-p <preset>`  
  プリセット（後述）を指定します。
* `-e <regexp>`  
  errorの行にマッチする[正規表現]を指定します。
* `-w <regexp>`  
  warningの行にマッチする[正規表現]を指定します。
* `-s <soundset>`  
  サウンドセット（後述）を指定します。
* `-stdout`  
  標準エラー出力の代わりに標準出力を読みます。


## プリセット

いちいち正規表現を書くのも面倒なので、上で作成されたErrorWarnerフォルダに`config.toml`という名前でテキストファイルを置いておけば、`-p`オプションでプリセットを呼び出せるようになります。

設定例：

```toml:config.toml
# 空文字列のプリセットを作るとデフォルト値として使われる
[preset.""]
stdout = true               # 標準エラー出力の代わりに標準出力を読む
errorFormat = '(?i:error)'  # errorの行にマッチする正規表現
warningFormat = '(?i:warn)' # warningの行にマッチする正規表現
soundset = 'boyacky'        # サウンドセット

# `errwarn -p tonzura` でこの設定を呼び出せるようになる
# 指定していない部分はデフォルト値が引き継がれる
[preset.tonzura]
soundset = 'tonzura'

# プリセットの名前をコマンドの名前（拡張子無し）にすると、
# コマンド渡しで実行したときに自動で選択してくれる
[preset.go]
stdout = false
errorFormat = '^.*: '
warningFormat = '' # 空の場合は使われない
```


## サウンドセット

複数のサウンドファイル群を切り替えたい場合のために、ErrorWarnerフォルダの下に`soundsets/*`フォルダを作ることでサウンドセットを作成することができます。

```
ErrorWarner
|-- config.toml
|-- error.wav
|-- warn.wav
|-- ...
|
+-- soundsets
    +-- boyacky
    |   |-- error.ogg
    |   |-- warn.ogg
    |   |-- ...
    |
    +-- tonzura
        |-- error.flac
        |-- warn.flac
        |-- ...
```

作ったサウンドセットは`-s`オプションやコンフィグの`soundset`パラメータで指定できます。
指定しなかった場合や空文字列を指定した場合はErrorWarnerフォルダ直下のサウンドファイルが使われます。

以下のファイル名のサウンドが使われます。

* `error.*`  
  errorが見つかったときに再生されます。
* `warn.*`  
  warningが見つかったときに再生されます。
* `start.*`  
  コマンドの開始時に再生されます。
* `finish.*`  
  errorやwarningがあったもののコマンドは正常終了した場合に再生されます。  
  パイプ渡しの場合はコマンドの終了時に常に再生されます。
* `success.*`  
  errorもwarningも無くコマンドが正常終了した場合に再生されます。
  パイプ渡しの場合は使われません。
* `fail.*`  
  コマンドが失敗した場合に再生されます。
  パイプ渡しの場合は使われません。


## ライセンス

[MIT](https://github.com/Bakudankun/ErrorWarner/blob/master/LICENSE)


Happy Erroring!!


[Oto]: https://github.com/hajimehoshi/oto
[Go]: https://golang.org/
[正規表現]: https://golang.org/pkg/regexp/syntax/

