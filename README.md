[![Go Report Card](https://goreportcard.com/badge/github.com/darktohka/ets2-dlc-repacker)](https://goreportcard.com/report/github.com/darktohka/ets2-dlc-repacker)
![](https://badges.fyi/github/license/darktohka/ets2-dlc-repacker)
![](https://badges.fyi/github/downloads/darktohka/ets2-dlc-repacker)
![](https://badges.fyi/github/latest-release/darktohka/ets2-dlc-repacker)

# darktohka / ets2-dlc-repacker

`ets2-dlc-repacker` is a Windows / Linux / MacOS CLI util to automatically repack older DLC archives for compatibility with newer versions.

This project uses a modified version of the [SCS file unpacker](https://github.com/Luzifer/scs-extract) written by [Luzifer](https://github.com/Luzifer).

## Usage

Simply 

`ets2-dlc-repacker [options] [game folder]`

```console
# ets2-dlc-repacker "~/.steam/steam/steamapps/common/Euro Truck Simulator 2"

# ets2-dlc-repacker --help
Usage of ets2-dlc-repacker:
      --log-level string   Log level (debug, info, warn, error, fatal) (default "info")
      --version            Prints current version and exits
```
