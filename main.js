const { app, Menu, Tray } = require('electron')
const HID = require('node-hid')
const path = require('path')
const iconPath = path.join(__dirname, 'battery-fill.ico')

let devices = HID.devices()
let steelseriesDevices = devices.filter(device => device.manufacturer == 'SteelSeries ' && device.usage !== 1)

function readDeviceBattery(devices) {
    let text = ''
    if (devices.length != 0) {
        devices.forEach((target) => {
            try {
                let device = new HID.HID(target.path)
                device.write([0x06, 0x18])
                let deviceData = device.readSync()
                if (deviceData[2] != 0) {
                    text += `${target.product}: ${deviceData[2]}%`
                } else {
                    text += `${target.product}: Disconnected or empty`
                }
                if (devices.length > 1) {
                    text += `\n`
                }    
                device.close()
            } catch (error) {
                console.log('Error while getting battery percentage: ', error)
            }
        })
        return text
    } else {
        text = 'No devices detected.'
        return text
    }
}

let tray = null
app.on('ready', function () {
    tray = new Tray(iconPath)

    const contextMenu = Menu.buildFromTemplate([
        {
            label: 'Exit',
            role: 'quit'
        }
    ])
    tray.setContextMenu(contextMenu)

    let update = false
    tray.on('mouse-move', function () {
        if (update != true) {
            update = true
            console.log(update)
            let tooltip = readDeviceBattery(steelseriesDevices)
            tray.setToolTip(tooltip)

            setTimeout(function() {
                update = false
                console.log(update)
            }, 10 * 1000);
        }
    })
})
