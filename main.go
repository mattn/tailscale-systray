package main

import (
	_ "embed"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
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

var (
	mu   sync.RWMutex
	myIP string
)

func main() {
	systray.Run(onReady, nil)
}

func executable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func doConnectionControl(m *systray.MenuItem, verbs ...string) {
	for {
		if _, ok := <-m.ClickedCh; !ok {
			break
		}
		b, err := exec.Command("pkexec", verbs...).CombinedOutput()
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
	mConnectExit := systray.AddMenuItem("Connect using exit node...", "")
	mConnectExit.Enable()
	mDisconnect := systray.AddMenuItem("Disconnect", "")
	mDisconnect.Disable()

	if executable("pkexec") {
		go doConnectionControl(mConnect,
			"tailscale", "up",
			"--exit-node", "",
			"--exit-node-allow-lan-access=false")
		go doConnectionControl(mDisconnect, "tailscale", "down")
	} else {
		mConnect.Hide()
		mConnectExit.Hide()
		mDisconnect.Hide()
	}

	systray.AddSeparator()

	mThisDevice := systray.AddMenuItem("This device:", "")
	go func(mThisDevice *systray.MenuItem) {
		for {
			_, ok := <-mThisDevice.ClickedCh
			if !ok {
				break
			}
			mu.RLock()
			if myIP == "" {
				mu.RUnlock()
				continue
			}
			err := clipboard.WriteAll(myIP)
			if err == nil {
				beeep.Notify(
					"This device",
					fmt.Sprintf("Copy the IP address (%s) to the Clipboard", myIP),
					"",
				)
			}
			mu.RUnlock()
		}
	}(mThisDevice)

	mNetworkDevices := systray.AddMenuItem("Network Devices", "")
	mMyDevices := mNetworkDevices.AddSubMenuItem("My Devices", "")
	mTailscaleServices := mNetworkDevices.AddSubMenuItem("Tailscale Services", "")

	systray.AddSeparator()
	mAdminConsole := systray.AddMenuItem("Admin Console...", "")
	go func() {
		for {
			_, ok := <-mAdminConsole.ClickedCh
			if !ok {
				break
			}
			openBrowser("https://login.tailscale.com/admin/machines")
		}
	}()

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
		exitNodes := map[string]*systray.MenuItem{}
		for {
			b, err := exec.Command("tailscale", "ip", "-4").Output()
			if err != nil {
				if enabled {
					systray.SetTooltip("Tailscale: Disconnected")
					mConnect.Enable()
					if len(exitNodes) == 0 {
						mConnectExit.Enable()
					} else {
						mConnectExit.Disable()
					}
					mDisconnect.Disable()
					systray.SetIcon(iconOff)
					enabled = false
				}
				time.Sleep(10 * time.Second)
				continue
			}

			mu.Lock()
			myIP = strings.TrimSpace(string(b))
			mu.Unlock()

			b, err = exec.Command("tailscale", "status").Output()
			if err != nil {
				if enabled {
					systray.SetTooltip("Tailscale: Disconnected")
					mConnect.Enable()
					if len(exitNodes) == 0 {
						mConnectExit.Enable()
					} else {
						mConnectExit.Disable()
					}
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
				if len(exitNodes) == 0 {
					// Stay enabled even when connected to allow
					// changing the exit node without having to reconnect
					mConnectExit.Enable()
				} else {
					mConnectExit.Disable()
				}
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

				// Show exit nodes
				if strings.Contains(line, "exit node") {
					_, ok := exitNodes[ip]
					if !ok {
						mConnectExitNode := mConnectExit.AddSubMenuItem(
							fmt.Sprintf("%s (%s)", title, ip), "")

						mConnectExitNode.Enable()
						if executable("pkexec") {
							go doConnectionControl(mConnectExitNode,
								"tailscale", "up",
								"--exit-node", string(ip),
								"--exit-node-allow-lan-access")
						}

						exitNodes[ip] = mConnectExitNode
					}
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

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("could not open link: %v", err)
	}
}
