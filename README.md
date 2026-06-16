# steelseries-wireless-battery-tray

A system tray (notification area) app that displays the battery percentage of SteelSeries wireless devices, rendered directly onto the tray icon.

I bought an Arctis 7 headset on Cyber Monday a while back and noticed there was no way to read the actual battery percentage, only a vague icon or the LED on the underside. So I made this simple tray app.

NOTE:
This was originally an Electron app, which I wrote a long time ago. After fixing some major issues it had, I have used AI to rewrite it in Go for a much smaller, native single-binary build. As a result, the binary has reduced from 90MB to about 5MB.

# releases

Prebuilt binaries for Windows, macOS, and Linux are attached to each [release](https://github.com/mtadin/steelseries-wireless-battery-tray/releases). Download the one for your OS and run it, the icon should appear in your tray. Click it to pick which device to display in case of multiple devices, or to quit.

The binaries are unsigned, and will remain that way unless I become a millionaire.

# building from source

Requires [Go](https://go.dev/dl/) 1.22+ and a C compiler (cgo is needed for HID and tray access).

```
go build -o steelseries-battery-tray .
```

Linux also needs the tray and HID development headers:

```
sudo apt-get install libayatana-appindicator3-dev libgtk-3-dev libudev-dev
```

# linux: HID permissions

On Linux, raw HID access needs a udev rule, otherwise the app reports "No devices detected." Create `/etc/udev/rules.d/99-steelseries.rules` with:

```
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1038", MODE="0660", TAG+="uaccess"
```

Then reload and replug the device:

```
sudo udevadm control --reload-rules && sudo udevadm trigger
```

# limitations

### The underlying HID library cannot read data from mice and keyboards, as macOS and Windows block reading those device types as a security precaution.

Tested with an Arctis 7 headset. Other SteelSeries wireless headsets that use the same battery read protocols should work with the new Go app, unless SteelSeries decides to change what is needed to read the battery percentage with a new generation of devices.

![example](https://github.com/mtadin/steelseries-wireless-battery-tray/blob/master/example.png?raw=true)
