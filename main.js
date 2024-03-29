const { app, Menu, Tray } = require("electron");
const fs = require("fs");
const HID = require("node-hid");
const Jimp = require("jimp");
const path = require("path");
const Store = require("electron-store");

const store = new Store();
const isMac = process.platform === "darwin";

const fontPath = path.join(__dirname, "Fipps-Regular.fnt");

const iconPath = path.join(__dirname, "icons", "battery-fill.png");
const editedIconPath = path.join(__dirname, "icons", "edited-icon.png");

let deviceList = new Map();

const readDeviceData = () => {
  const devices = HID.devices().filter(
    (device) =>
      device.manufacturer &&
      device.manufacturer.toUpperCase().includes("STEELSERIES") &&
      device.usage !== 1
  );

  if (devices.length > 0) {
    devices.forEach(({ path, product }) => {
      if (path) {
        try {
          const device = new HID.HID(path);

          let deviceData = [];

          try {
            device.write([0x06, 0x18]);
            deviceData = device.readSync();

            if (deviceData[2] > 0) {
              deviceList.set(product, `${String(deviceData[2])}%`);
            } else {
              deviceList.set(product, `xx`);
            }
          } catch (e) {
            console.error("Error while writing data:", e);
            device.close();
          }

          device.close();
        } catch (error) {
          console.error("Error while getting battery percentage: ", error);
        }
      }
    });
  } else {
    deviceList.clear();
    deviceList.set("No devices detected.", "");
  }
};

const buildContextMenu = (tray) => {
  const deviceToDisplay =
    store.get("display") || deviceList.keys().next().value;

  tray.setContextMenu(
    Menu.buildFromTemplate([
      ...Array.from(deviceList, ([product, value]) => {
        const selected = deviceToDisplay === product;

        return {
          label: `${selected ? "> " : ""}${product}: ${
            value == "xx" ? "Disconnected" : value
          }`,
          click: () => {
            store.set("display", product);
            buildContextMenu(tray);
          },
        };
      }),
      {
        label: "Quit",
        role: isMac ? "close" : "quit",
      },
    ])
  );
};

const updateTrayIcon = async () => {
  const deviceToDisplay =
    store.get("display") || deviceList.keys().next().value;

  await Jimp.read(iconPath, (error, image) => {
    if (error) throw error;

    Jimp.loadFont(fontPath).then((font) => {
      image.print(font, 4, 4, deviceList.get(deviceToDisplay));
      image.write(editedIconPath);
    });
  });
};

const mainFunction = (tray) => {
  readDeviceData();
  buildContextMenu(tray);
  updateTrayIcon();
  tray.setImage(fs.existsSync(editedIconPath) ? editedIconPath : iconPath);
};

app.on("ready", async () => {
  const tray = new Tray(iconPath);
  mainFunction(tray);

  setInterval(() => {
    mainFunction(tray);
  }, 60000);
});
