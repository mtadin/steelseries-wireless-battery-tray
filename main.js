const { app, Menu, Tray } = require("electron");
const fs = require("fs");
const HID = require("node-hid");
const Jimp = require("jimp");
const path = require("path");
const Store = require("electron-store");

const store = new Store();

const fontPath = path.join(__dirname, "Fipps-Regular.fnt");
const iconPath = path.join(__dirname, "icons", "battery-fill.png");
const editedIconPath = path.join(app.getPath("userData"), "edited-icon.png");

let deviceList = new Map();

const READ_TIMEOUT = 500;
const PROTOCOLS = [
  { command: [0x06, 0x12], byte: 3 }, // newer headsets, try first
  { command: [0x06, 0x18], byte: 2 }, // older headsets, fallback
];
const protocolCache = new Map();

const tryProtocol = (device, protocol) => {
  device.write(protocol.command);
  const data = device.readTimeout(READ_TIMEOUT);
  return data.length && data[protocol.byte] > 0 ? data[protocol.byte] : null;
};

const readBattery = (device, devicePath) => {
  const cachedIndex = protocolCache.get(devicePath);
  const order =
    cachedIndex != null
      ? [cachedIndex, ...[...PROTOCOLS.keys()].filter((i) => i !== cachedIndex)]
      : [...PROTOCOLS.keys()];

  for (const i of order) {
    const result = tryProtocol(device, PROTOCOLS[i]);
    if (result != null) {
      protocolCache.set(devicePath, i);
      return result;
    }
  }
  return null;
};

const readDeviceData = () => {
  const devices = HID.devices().filter(
    (device) =>
      device.manufacturer &&
      device.manufacturer.toUpperCase().includes("STEELSERIES") &&
      device.usage !== 1,
  );

  if (devices.length === 0) {
    deviceList = new Map([["No devices detected.", ""]]);
    return;
  }

  const next = new Map();
  devices.forEach(({ path: devicePath, product }) => {
    if (!devicePath) return;

    let device;
    try {
      device = new HID.HID(devicePath);
      const battery = readBattery(device, devicePath);
      next.set(product, battery != null ? `${battery}%` : "xx");
    } catch (error) {
      console.error("Error while reading battery percentage:", error);
    } finally {
      if (device) {
        try {
          device.close();
        } catch (error) {
          console.error("Error while closing device:", error);
        }
      }
    }
  });

  deviceList = next;
};

const resolveDisplay = () => {
  const stored = store.get("display");
  return stored && deviceList.has(stored)
    ? stored
    : deviceList.keys().next().value;
};

const buildContextMenu = (tray) => {
  const deviceToDisplay = resolveDisplay();

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
        role: "quit",
      },
    ]),
  );
};

const updateTrayIcon = async () => {
  const text = deviceList.get(resolveDisplay());
  if (!text) return;

  const [image, font] = await Promise.all([
    Jimp.read(iconPath),
    Jimp.loadFont(fontPath),
  ]);

  image.print(font, 4, 4, text);
  await image.writeAsync(editedIconPath);
};

const mainFunction = async (tray) => {
  readDeviceData();
  buildContextMenu(tray);
  await updateTrayIcon();
  tray.setImage(fs.existsSync(editedIconPath) ? editedIconPath : iconPath);
};

app.on("ready", () => {
  const tray = new Tray(iconPath);
  mainFunction(tray);

  setInterval(() => {
    mainFunction(tray);
  }, 60000);
});
