#!/usr/bin/env python3
import os, time, select, struct, glob

REPORT_SIZE = 3   # /dev/input/mouseX gives 3-byte packets
POLL_INTERVAL = 0.01
HOTPLUG_SCAN_INTERVAL = 2.0   # seconds between rescans

def decode_mice_packet(data: bytes):
    if len(data) < 3:
        return None
    buttons = data[0]
    dx = struct.unpack("b", data[1:2])[0]  # signed 8-bit
    dy = struct.unpack("b", data[2:3])[0]  # signed 8-bit

    pressed = []
    if buttons & 0x1: pressed.append("L")
    if buttons & 0x2: pressed.append("R")
    if buttons & 0x4: pressed.append("M")

    return f"buttons={pressed if pressed else 'None'} dx={dx} dy={dy}"


class MouseDevice:
    def __init__(self, path):
        self.path = path
        self.fd = None
        self.open()

    def open(self):
        try:
            self.fd = os.open(self.path, os.O_RDONLY | os.O_NONBLOCK)
            print(f"[+] Opened {self.path}")
        except Exception as e:
            self.fd = None

    def close(self):
        if self.fd is not None:
            try:
                os.close(self.fd)
            except:
                pass
            print(f"[-] Closed {self.path}")
            self.fd = None

    def fileno(self):
        return self.fd if self.fd is not None else None

    def read_event(self):
        if self.fd is None:
            return None
        try:
            data = os.read(self.fd, REPORT_SIZE)
            if not data:
                return None
            return decode_mice_packet(data)
        except BlockingIOError:
            return None
        except OSError:
            # device vanished
            self.close()
            return None


def monitor_mice():
    devices = {}
    last_scan = 0

    while True:
        now = time.time()
        if now - last_scan > HOTPLUG_SCAN_INTERVAL:
            last_scan = now
            # scan for /dev/input/mouse* and /dev/input/mice
            candidates = glob.glob("/dev/input/mouse*")
            if os.path.exists("/dev/input/mice"):
                candidates.append("/dev/input/mice")

            found = set(candidates)

            # add new devices
            for path in candidates:
                if path not in devices:
                    dev = MouseDevice(path)
                    if dev.fd is not None:
                        devices[path] = dev

            # remove vanished
            for path in list(devices.keys()):
                if path not in found:
                    devices[path].close()
                    del devices[path]

        if devices:
            try:
                rlist, _, _ = select.select(
                    [d.fileno() for d in devices.values() if d.fileno() is not None],
                    [], [], POLL_INTERVAL
                )
                for fd in rlist:
                    dev = next((d for d in devices.values() if d.fileno() == fd), None)
                    if not dev:
                        continue
                    out = dev.read_event()
                    if out:
                        ts = int(time.time()*1000)
                        print(f"[{ts} ms] {os.path.basename(dev.path)}: {out}")
            except Exception:
                pass
        else:
            time.sleep(POLL_INTERVAL)  # avoid busy loop if nothing connected


if __name__ == "__main__":
    try:
        monitor_mice()
    except KeyboardInterrupt:
        print("\nStopped.")
