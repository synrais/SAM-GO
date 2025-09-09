#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <linux/joystick.h>
#include <linux/input.h>
#include <sys/ioctl.h>
#include <poll.h>

#define MAX_DEVICES 16

struct js_device {
    int fd;
    char name[128];
    int axes;
    int buttons;
    __u8 axmap[ABS_MAX + 1];
    __u16 btnmap[KEY_MAX - BTN_MISC + 1];
};

static struct js_device devices[MAX_DEVICES];
static int device_count = 0;

void spy_scan_devices() {
    char path[64];
    for (int i = 0; i < MAX_DEVICES; i++) {
        snprintf(path, sizeof(path), "/dev/input/js%d", i);
        int fd = open(path, O_RDONLY | O_NONBLOCK);
        if (fd < 0) continue;

        struct js_device dev;
        memset(&dev, 0, sizeof(dev));
        dev.fd = fd;
        ioctl(fd, JSIOCGNAME(sizeof(dev.name)), dev.name);
        ioctl(fd, JSIOCGAXES, &dev.axes);
        ioctl(fd, JSIOCGBUTTONS, &dev.buttons);
        ioctl(fd, JSIOCGAXMAP, dev.axmap);
        ioctl(fd, JSIOCGBTNMAP, dev.btnmap);

        devices[device_count++] = dev;

        printf("Monitoring %s (%s)\n", path, dev.name);
        printf("  Axes: %d  Buttons: %d\n", dev.axes, dev.buttons);
    }
}

void spy_loop() {
    struct pollfd fds[MAX_DEVICES];
    for (int i = 0; i < device_count; i++) {
        fds[i].fd = devices[i].fd;
        fds[i].events = POLLIN;
    }

    while (1) {
        int ret = poll(fds, device_count, 1000);
        if (ret <= 0) continue;

        for (int i = 0; i < device_count; i++) {
            if (!(fds[i].revents & POLLIN)) continue;

            struct js_event e;
            while (read(fds[i].fd, &e, sizeof(e)) > 0) {
                e.type &= ~JS_EVENT_INIT;
                if (e.type == JS_EVENT_AXIS) {
                    printf("[%s] Axis %d = %d\n", devices[i].name, e.number, e.value);
                } else if (e.type == JS_EVENT_BUTTON) {
                    printf("[%s] Button %d = %d\n", devices[i].name, e.number, e.value);
                }
                fflush(stdout);
            }
        }
    }
}
