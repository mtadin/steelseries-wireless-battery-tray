# steelseries-wireless-battery-tray

A system tray (or notification area) app that displays battery percentages of Steelseries wireless devices.

I've bought an Arctis 7 headset on Cyber Monday a while back, and noticed that there is no way to get the actual battery percentage of the headset, except for a vague icon or the LED on the underside of the headset. So, as a little project I made this system tray application that should display the battery percentages of Steelseries wireless headsets when you hover you mouse over it.

Currently only tested on Windows 10 with an Arctis 7 headset. If you have other devices or operating system, contact me to expand this.

# [releases](https://github.com/mtadin/steelseries-wireless-battery-tray/releases)

Currently only a Windows .zip is available. Unzip it at a location of your choice and [add the .exe to your startup folder.](https://support.microsoft.com/en-us/windows/add-an-app-to-run-automatically-at-startup-in-windows-10-150da165-dcd9-7230-517b-cf3c295d89dd)

### beta release

The releases tab has a 2.0.0-beta version which includes a dynamic icon that displays the percentage for the chosen device on the icon itself.

Someday when I rewrite the app and add tests I will also add releases for other operating systems. Someday.

# limitations

The underlying library used to read device data unfortunately cannot read data from [mice and keyboards due to a security precaution.](https://github.com/node-hid/node-hid#devices-node-hid-cannot-read)

# installation

Requires [node.js](https://nodejs.org/en/) installed on your computer.

After downloading the project, navigate to the folder containing it and run these commands in your terminal:

```
npm install
```

and

```
npm run make
```

When it finishes compiling, navigate to the 'out/steelseries-wireless-battery-tray' folder and run the .exe, or [add it to your startup folder](https://support.microsoft.com/en-us/windows/add-an-app-to-run-automatically-at-startup-in-windows-10-150da165-dcd9-7230-517b-cf3c295d89dd).

The battery icon should appear in your system tray, and it should display a tooltip containing the battery percentage of your connected device. If you have multiple devices, clicking on the icon brings up a menu which allows you to choose which device should be displayed, or to exit and turn off the application.

![example](https://github.com/mtadin/steelseries-wireless-battery-tray/blob/master/example.png?raw=true)
