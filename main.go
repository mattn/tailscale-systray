package main

import (
	_ "embed"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gen2brain/beeep"
	"github.com/getlantern/systray"
)

var (
	//go:embed icon/on.png
	iconOn []byte
	//go:embed icon/off.png
	iconOff []byte
)

func main() {
	systray.Run(onReady, nil)
}

func doConnectionControl(m *systray.MenuItem, verb string) {
	for {
		if _, ok := <-m.ClickedCh; !ok {
			break
		}
		b, err := exec.Command("pkexec", "tailscale", verb).CombinedOutput()
		if err != nil {
			beeep.Notify(
				"Tailscale",
				string(b),
				"",
			)
		}
	}
}

func onReady() {
	systray.SetIcon(iconOff)

	mConnect := systray.AddMenuItem("Connect", "")
	mConnect.Enable()
	go doConnectionControl(mConnect, "up")
	mDisconnect := systray.AddMenuItem("Disconnect", "")
	mDisconnect.Disable()
	go doConnectionControl(mConnect, "down")

	systray.AddSeparator()

	mThisDevice := systray.AddMenuItem("This device:", "")
	mNetworkDevices := systray.AddMenuItem("Network Devices", "")
	mMyDevices := mNetworkDevices.AddSubMenuItem("My Devices", "")
	mTailscaleServices := mNetworkDevices.AddSubMenuItem("Tailscale Services", "")
	systray.AddSeparator()
	mExit := systray.AddMenuItem("Exit", "")
	go func() {
		<-mExit.ClickedCh
		systray.Quit()
	}()

	go func() {
		type Item struct {
			menu  *systray.MenuItem
			title string
			ip    string
			found bool
		}
		items := map[string]*Item{}
		enabled := false
		for {
			b, err := exec.Command("tailscale", "ip", "-4").Output()
			if err != nil {
				if enabled {
					systray.SetTooltip("Tailscale: Disconnected")
					mConnect.Enable()
					mDisconnect.Disable()
					systray.SetIcon(iconOff)
					enabled = false
				}
				time.Sleep(10 * time.Second)
				continue
			}
			myIP := strings.TrimSpace(string(b))

			b, err = exec.Command("tailscale", "status").Output()
			if err != nil {
				if enabled {
					systray.SetTooltip("Tailscale: Disconnected")
					mConnect.Enable()
					mDisconnect.Disable()
					systray.SetIcon(iconOff)
					enabled = false
					time.Sleep(time.Second)
				}
				continue
			}
			if !enabled {
				systray.SetTooltip("Tailscale: Connected")
				mConnect.Disable()
				mDisconnect.Enable()
				systray.SetIcon(iconOn)
				enabled = true
			}

			for _, v := range items {
				v.found = false
			}
			for _, line := range strings.Split(string(b), "\n") {
				fields := strings.Fields(line)
				if len(fields) == 0 {
					continue
				}

				ip := fields[0]
				title := fields[1]

				if ip == myIP {
					mThisDevice.SetTitle(fmt.Sprintf("This device: %s (%s)", title, ip))
					continue
				}

				var sub *systray.MenuItem
				if strings.HasPrefix(title, "(") {
					title = strings.Trim(title, `()"`)
					sub = mTailscaleServices
				} else {
					sub = mMyDevices
				}

				if item, ok := items[title]; ok {
					item.found = true
				} else {
					items[title] = &Item{
						menu:  sub.AddSubMenuItem(title, title),
						title: title,
						ip:    ip,
						found: true,
					}
					go func(item *Item) {
						// TODO fix race condition
						for {
							_, ok := <-item.menu.ClickedCh
							if !ok {
								break
							}
							err := clipboard.WriteAll(item.ip)
							if err != nil {
								beeep.Notify(
									"Tailscale",
									string(b),
									"",
								)
								return
							}
							beeep.Notify(
								item.title,
								fmt.Sprintf("Copy the IP address (%s) to the Clipboard", item.ip),
								"",
							)
						}
					}(items[title])
				}
			}

			for k, v := range items {
				if !v.found {
					// TODO fix race condition
					v.menu.Hide()
					delete(items, k)
				}
			}

			time.Sleep(10 * time.Second)
		}
	}()
}
