# tailscale-systray

Linux port of tailscale system tray menu.

![tailscale-systray](/screenshot.png)

## Usage

```
$ tailscale-systray
```

## Requirements

* tailscale

## Installation

### Install requirements

Building app requires gcc, libgtk-3-dev, libayatana-appindicator3-dev

```
sudo apt-get install gcc libgtk-3-dev libayatana-appindicator3-dev
```

### Install app

```
go install -v github.com/mattn/tailscale-systray@latest
```

At this point you can start it with `tailscale-systray`. The executable should be located in `$GOPATH/bin` (if not explicitly set, `$GOPATH` defaults to `$HOME/go` on Linux).

### Run at startup

You can do this in different ways. One option is to add the command to "Startup Applications" if your system has it:

![tailscale-systray startup](/screenshot_startup.png)

## License

MIT

Icon file is copied from official repository.

## Author

Yasuhiro Matsumoto
