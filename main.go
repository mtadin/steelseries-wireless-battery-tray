package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"
	ico "github.com/Kodeworks/golang-image-ico"
	"github.com/sstallion/go-hid"
)

//go:embed icons/battery-fill.png
var baseIconPNG []byte

//go:embed Fipps-Regular.fnt
var fippsFnt []byte

//go:embed Fipps-Regular.png
var fippsPNG []byte

const (
	pollInterval = 60 * time.Second
	readTimeout  = 500 * time.Millisecond
	maxDevices   = 8 // size of the reusable systray menu-item pool
	noDevices    = "No devices detected."
)

// protocol describes a battery-query command and the byte in the reply that
// carries the battery value. Two are tried in order; see tryProtocol.
type protocol struct {
	command []byte
	byteIdx int
}

var protocols = []protocol{
	{command: []byte{0x06, 0x12}, byteIdx: 3}, // newer headsets, try first
	{command: []byte{0x06, 0x18}, byteIdx: 2}, // older headsets, fallback
}

// Remembers which protocol last worked for a device path. Only touched from
// the poll goroutine, so it needs no lock.
var protocolCache = map[string]int{}

type deviceEntry struct {
	product string
	value   string // "85%", or "xx" when connected-but-no-reading
}

type poolEntry struct {
	item    *systray.MenuItem
	product string // device this slot currently represents ("" if unused)
}

var (
	mu         sync.Mutex
	deviceList []deviceEntry
	selected   string // product chosen for the icon/tooltip
	pool       []*poolEntry
)

// tryProtocol writes a query and returns the battery value, or false if this
// protocol produced no reading. Note: a reported 0 is treated as "no reading"
// because a wrong protocol answers with 0 at this byte; the side effect is
// that a genuine 0% battery reads as disconnected. Carried over from the
// original app deliberately, distinguishing the two needs device testing.
func tryProtocol(d *hid.Device, p protocol) (int, bool) {
	if _, err := d.Write(p.command); err != nil {
		return 0, false
	}
	buf := make([]byte, 64)
	n, err := d.ReadWithTimeout(buf, readTimeout)
	if err != nil || n == 0 || p.byteIdx >= n {
		return 0, false
	}
	if v := int(buf[p.byteIdx]); v > 0 {
		return v, true
	}
	return 0, false
}

func readBattery(path string) int {
	d, err := hid.OpenPath(path)
	if err != nil {
		return -1
	}
	defer d.Close()

	// Try the cached protocol first, then the rest.
	order := []int{0, 1}
	if cached, ok := protocolCache[path]; ok {
		order = []int{cached}
		for i := range protocols {
			if i != cached {
				order = append(order, i)
			}
		}
	}

	for _, i := range order {
		if v, ok := tryProtocol(d, protocols[i]); ok {
			protocolCache[path] = i
			return v
		}
	}
	return -1
}

// readDeviceData enumerates SteelSeries HID interfaces and reads each one's
// battery. Rebuilt from scratch every poll so unplugged devices drop out.
func readDeviceData() []deviceEntry {
	var order []string
	vals := map[string]string{}

	hid.Enumerate(0, 0, func(info *hid.DeviceInfo) error {
		if info.Path == "" || info.Usage == 1 ||
			!strings.Contains(strings.ToUpper(info.MfrStr), "STEELSERIES") {
			return nil
		}

		v := "xx"
		if battery := readBattery(info.Path); battery >= 0 {
			v = fmt.Sprintf("%d%%", battery)
		}

		// A device exposes several interfaces; key by product and prefer a
		// real reading over "xx".
		if _, seen := vals[info.ProductStr]; !seen {
			order = append(order, info.ProductStr)
		}
		if old, seen := vals[info.ProductStr]; !seen || (old == "xx" && v != "xx") {
			vals[info.ProductStr] = v
		}
		return nil
	})

	if len(order) == 0 {
		return []deviceEntry{{product: noDevices}}
	}
	out := make([]deviceEntry, len(order))
	for i, p := range order {
		out[i] = deviceEntry{product: p, value: vals[p]}
	}
	return out
}

// resolveDisplayLocked returns the stored selection if still present, else the
// first device. Caller must hold mu.
func resolveDisplayLocked(devices []deviceEntry) string {
	for _, d := range devices {
		if d.product == selected {
			return selected
		}
	}
	if len(devices) > 0 {
		return devices[0].product
	}
	return ""
}

func displayValue(v string) string {
	if v == "xx" {
		return "Disconnected"
	}
	return v
}

func menuLabel(d deviceEntry, isSelected bool) string {
	prefix := ""
	if isSelected {
		prefix = "> "
	}
	if d.value == "" {
		return prefix + d.product
	}
	return fmt.Sprintf("%s%s: %s", prefix, d.product, displayValue(d.value))
}

// render maps current state onto the tray: menu items, tooltip and icon.
// systray cannot remove menu items, so a fixed pool is shown/hidden instead.
func render() {
	mu.Lock()
	devices := make([]deviceEntry, len(deviceList))
	copy(devices, deviceList)
	sel := resolveDisplayLocked(devices)
	selected = sel
	for i, pe := range pool {
		if i < len(devices) {
			pe.product = devices[i].product
		} else {
			pe.product = ""
		}
	}
	mu.Unlock()

	for i, pe := range pool {
		if i < len(devices) {
			pe.item.SetTitle(menuLabel(devices[i], devices[i].product == sel))
			pe.item.Show()
		} else {
			pe.item.Hide()
		}
	}

	val := ""
	for _, d := range devices {
		if d.product == sel {
			val = d.value
		}
	}

	if sel == "" || sel == noDevices {
		systray.SetTooltip("SteelSeries Battery")
	} else {
		systray.SetTooltip(fmt.Sprintf("%s: %s", sel, displayValue(val)))
	}

	if icon, err := renderIcon(val); err == nil {
		systray.SetIcon(icon)
	} else {
		log.Println("render icon:", err)
	}
}

// glyph is one character's rectangle in the bitmap-font atlas plus its
// placement offsets, parsed from the BMFont .fnt file.
type glyph struct {
	x, y, w, h, xoff, yoff, xadv int
}

var (
	glyphs    = map[rune]glyph{}
	fontAtlas image.Image
)

// init parses the embedded Fipps bitmap font (same font the original Electron
// app used) so glyphs can be blitted onto the icon.
func init() {
	img, err := png.Decode(bytes.NewReader(fippsPNG))
	if err != nil {
		panic(err)
	}
	fontAtlas = img

	sc := bufio.NewScanner(bytes.NewReader(fippsFnt))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "char ") {
			continue
		}
		f := map[string]int{}
		for _, tok := range strings.Fields(line)[1:] {
			if k, v, ok := strings.Cut(tok, "="); ok {
				n, _ := strconv.Atoi(v)
				f[k] = n
			}
		}
		glyphs[rune(f["id"])] = glyph{
			f["x"], f["y"], f["width"], f["height"],
			f["xoffset"], f["yoffset"], f["xadvance"],
		}
	}
}

// drawText reproduces jimp's image.print for a bitmap font, pixel-for-pixel as
// the original JS app did: blit each glyph from the atlas at its native size to
// (x+xoffset, y+yoffset), alpha-composited over the icon, advancing by
// xadvance. No kerning (the font has none), no scaling. The '%' glyph is
// zero-width, so it just advances, exactly like the original.
func drawText(dst *image.RGBA, x, y int, text string) {
	pen := x
	for _, r := range text {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		for gy := 0; gy < g.h; gy++ {
			for gx := 0; gx < g.w; gx++ {
				sr, sg, sb, sa := fontAtlas.At(g.x+gx, g.y+gy).RGBA()
				if sa == 0 {
					continue
				}
				dx, dy := pen+g.xoff+gx, y+g.yoff+gy
				a := float64(sa) / 0xffff
				dc := dst.RGBAAt(dx, dy)
				dst.SetRGBA(dx, dy, color.RGBA{
					R: uint8(float64(sr>>8)*a + float64(dc.R)*(1-a)),
					G: uint8(float64(sg>>8)*a + float64(dc.G)*(1-a)),
					B: uint8(float64(sb>>8)*a + float64(dc.B)*(1-a)),
					A: uint8(float64(sa>>8) + float64(dc.A)*(1-a)),
				})
			}
		}
		pen += g.xadv
	}
}

// renderIcon draws the battery text onto the embedded base icon, in memory.
// Windows needs ICO bytes; macOS/Linux take PNG.
func renderIcon(text string) ([]byte, error) {
	base, err := png.Decode(bytes.NewReader(baseIconPNG))
	if err != nil {
		return nil, err
	}
	b := base.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, base, b.Min, draw.Src)

	if text != "" {
		drawText(rgba, 4, 4, text)
	}

	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		err = ico.Encode(&buf, rgba)
	} else {
		err = png.Encode(&buf, rgba)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func poll() {
	devices := readDeviceData()
	mu.Lock()
	deviceList = devices
	mu.Unlock()
	render()
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "steelseries-battery-tray", "config.json")
}

func loadConfig() string {
	b, err := os.ReadFile(configPath())
	if err != nil {
		return ""
	}
	var c struct {
		Display string `json:"display"`
	}
	json.Unmarshal(b, &c)
	return c.Display
}

func saveConfig(display string) {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		log.Println("save config:", err)
		return
	}
	b, _ := json.Marshal(struct {
		Display string `json:"display"`
	}{display})
	if err := os.WriteFile(p, b, 0o644); err != nil {
		log.Println("save config:", err)
	}
}

func onReady() {
	if icon, err := renderIcon(""); err == nil {
		systray.SetIcon(icon)
	}
	systray.SetTooltip("SteelSeries Battery")

	for i := 0; i < maxDevices; i++ {
		mi := systray.AddMenuItem("", "")
		mi.Hide()
		pe := &poolEntry{item: mi}
		pool = append(pool, pe)

		go func(pe *poolEntry) {
			for range pe.item.ClickedCh {
				mu.Lock()
				product := pe.product
				if product != "" && product != noDevices {
					selected = product
				}
				mu.Unlock()
				if product != "" && product != noDevices {
					saveConfig(product)
					render()
				}
			}
		}(pe)
	}

	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Quit")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()

	poll()
	go func() {
		t := time.NewTicker(pollInterval)
		defer t.Stop()
		for range t.C {
			poll()
		}
	}()
}

func main() {
	if err := hid.Init(); err != nil {
		log.Fatal("hid init:", err)
	}
	defer hid.Exit()

	mu.Lock()
	selected = loadConfig()
	mu.Unlock()

	systray.Run(onReady, func() {})
}
