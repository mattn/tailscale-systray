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

func executable(command string) bool {
	_, err := exec.LookPath("pkexec")
	return err == nil
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
	mConnect.Show()
	mDisconnect := systray.AddMenuItem("Disconnect", "")
	mDisconnect.Hide()

	if executable("pkexec") {
		go doConnectionControl(mConnect, "up")
		go doConnectionControl(mDisconnect, "down")
	} else {
		mConnect.Hide()
		mDisconnect.Hide()
	}

	systray.AddSeparator()

	mThisDevice := systray.AddMenuItem("This device:", "")
	mNetworkDevices := systray.AddMenuItem("Network Devices", "")
	mMyDevices := mNetworkDevices.AddSubMenuItem("My Devices", "")
	mTailscaleServices := mNetworkDevices.AddSubMenuItem("Tailscale Services", "")

	if executable("x-www-browser") {
		systray.AddSeparator()
		mAdminConsole := systray.AddMenuItem("Admin Console...", "")
		go func() {
			for {
				_, ok := <-mAdminConsole.ClickedCh
				if !ok {
					break
				}
				exec.Command("x-www-browser", "https://login.tailscale.com/admin/machines").Start()
			}
		}()
	}

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
					mConnect.Show()
					mDisconnect.Hide()
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
					mConnect.Show()
					mDisconnect.Hide()
					systray.SetIcon(iconOff)
					enabled = false
					time.Sleep(time.Second)
				}
				continue
			}
			if !enabled {
				systray.SetTooltip("Tailscale: Connected")
				mConnect.Hide()
				mDisconnect.Show()
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
