#!/usr/bin/env python3
import sys
import os
import glob
import select
import time

HOTPLUG_SCAN_INTERVAL = 2.0   # seconds between rescans

# --- Load SCAN_CODES from external file (scan_codes.txt) ---
SCAN_CODES = {}
here = os.path.dirname(os.path.abspath(__file__))
scan_file = os.path.join(here, "keyboardscancodes.txt")
if not os.path.exists(scan_file):
    print(f"Error: {scan_file} not found")
    sys.exit(1)

with open(scan_file, "r") as f:
    code_env = {}
    exec(f.read(), code_env)
    SCAN_CODES = code_env.get("SCAN_CODES", {})


def parse_keyboards():
    """Return dict {sysfs_id: name} for all keyboards from /proc/bus/input/devices."""
    keyboards = {}
    block = []
    with open("/proc/bus/input/devices") as f:
        for line in f:
            if line.strip() == "":
                if any("Handlers=" in l and "kbd" in l for l in block):
                    name_line = next((l for l in block if l.startswith("N: ")), None)
                    sysfs_line = next((l for l in block if l.startswith("S: Sysfs=")), None)
                    if name_line and sysfs_line:
                        name = name_line.split("=", 1)[1].strip().strip('"')
                        sysfs_path = sysfs_line.split("=", 1)[1].strip()
                        sysfs_id = None
                        parts = sysfs_path.split("/")
                        for p in reversed(parts):
                            if p.startswith("0003:"):
                                sysfs_id = p
                                break
                        if sysfs_id:
                            keyboards[sysfs_id] = name
                block = []
            else:
                block.append(line)
    return keyboards


def match_hidraws(keyboards):
    """Match keyboards sysfs IDs to /dev/hidrawX devices. Return list of (devnode, name)."""
    matches = []
    for hiddev in glob.glob("/sys/class/hidraw/hidraw*/device"):
        target = os.path.realpath(hiddev)
        sysfs_id = os.path.basename(target)
        if sysfs_id in keyboards:
            devnode = f"/dev/{os.path.basename(os.path.dirname(hiddev))}"
            matches.append((devnode, keyboards[sysfs_id]))
    return matches


def decode_report(report):
    """Decode only real keystrokes (ignores touchpad/mouse junk)."""
    if len(report) != 8:
        return None
    if report[0] == 0x02:
        return None
    if report[0] != 0 and all(b == 0 for b in report[1:]):
        return None

    keycodes = report[2:8]
    output = []
    for code in keycodes:
        if code == 0:
            continue
        if code in SCAN_CODES:
            key = SCAN_CODES[code][0]
            if key == "SPACE":
                output.append(" ")
            elif key == "ENTER":
                output.append("\n")
            elif len(key) == 1:
                output.append(key)
            else:
                output.append(f"<{key}>")
    return "".join(output) if output else None


class KeyboardDevice:
    def __init__(self, devnode, name):
        self.devnode = devnode
        self.name = name
        self.fd = None
        self.open()

    def open(self):
        try:
            self.fd = open(self.devnode, "rb", buffering=0)
            print(f"[+] Opened {self.devnode} → {self.name}")
        except Exception as e:
            self.fd = None

    def close(self):
        if self.fd:
            try:
                self.fd.close()
            except:
                pass
            self.fd = None
            print(f"[-] Closed {self.devnode} → {self.name}")

    def fileno(self):
        return self.fd.fileno() if self.fd else None

    def read_event(self):
        if not self.fd:
            return None
        try:
            data = self.fd.read(8)
            if not data:
                return None
            return decode_report(data)
        except Exception:
            # device gone
            self.close()
            return None


def monitor_keyboards():
    devices = {}
    last_scan = 0

    while True:
        now = time.time()
        if now - last_scan > HOTPLUG_SCAN_INTERVAL:
            last_scan = now
            # rescan for keyboards
            keyboards = parse_keyboards()
            matches = match_hidraws(keyboards)
            found = set(dev for dev, _ in matches)

            # Add new devices
            for devnode, name in matches:
                if devnode not in devices:
                    dev = KeyboardDevice(devnode, name)
                    if dev.fd:
                        devices[devnode] = dev

            # Remove vanished devices
            for devnode in list(devices.keys()):
                if devnode not in found:
                    devices[devnode].close()
                    del devices[devnode]

        if devices:
            try:
                rlist, _, _ = select.select([d.fileno() for d in devices.values() if d.fd], [], [], 0.2)
                for fd in rlist:
                    dev = next((d for d in devices.values() if d.fileno() == fd), None)
                    if not dev:
                        continue
                    out = dev.read_event()
                    if out:
                        sys.stdout.write(out)
                        sys.stdout.flush()
            except Exception:
                pass
        else:
            time.sleep(0.2)  # avoid busy loop if nothing connected


if __name__ == "__main__":
    try:
        monitor_keyboards()
    except KeyboardInterrupt:
        print("\n[+] Exiting.")
