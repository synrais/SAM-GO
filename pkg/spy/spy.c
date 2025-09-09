#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <linux/joystick.h>
#include "axbtnmap.h"

#define MAX_DEVICES 16

typedef struct {
    int fd;
    char name[128];
    unsigned char axes;
    unsigned char buttons;
    uint8_t axmap[AXMAP_SIZE];
    uint16_t btnmap[BTNMAP_SIZE];
    int axis_state[AXMAP_SIZE];
    int button_state[BTNMAP_SIZE];
} js_device;

static js_device devices[MAX_DEVICES];
static int dev_count = 0;

void spy_scan_devices() {
    char path[64];
    for (int i = 0; i < MAX_DEVICES; i++) {
        snprintf(path, sizeof(path), "/dev/input/js%d", i);
        int fd = open(path, O_RDONLY | O_NONBLOCK);
        if (fd < 0) continue;

        js_device *dev = &devices[dev_count++];
        dev->fd = fd;
        ioctl(fd, JSIOCGNAME(sizeof(dev->name)), dev->name);
        ioctl(fd, JSIOCGAXES, &dev->axes);
        ioctl(fd, JSIOCGBUTTONS, &dev->buttons);
        getaxmap(fd, dev->axmap);
        getbtnmap(fd, dev->btnmap);

        printf("Monitoring %s (%s)\n", path, dev->name);
        printf("  Axes: %d  Buttons: %d\n", dev->axes, dev->buttons);
    }
}

void spy_loop() {
    struct js_event e;
    while (1) {
        for (int d = 0; d < dev_count; d++) {
            js_device *dev = &devices[d];
            while (read(dev->fd, &e, sizeof(e)) > 0) {
                e.type &= ~JS_EVENT_INIT;
                if (e.type == JS_EVENT_AXIS) {
                    int axis = dev->axmap[e.number];
                    dev->axis_state[axis] = e.value;
                } else if (e.type == JS_EVENT_BUTTON) {
                    int btn = dev->btnmap[e.number];
                    dev->button_state[btn] = e.value;
                }
            }

            // Full RetroSpy-style dump
            printf("[%s] Axes[", dev->name);
            for (int i = 0; i < dev->axes; i++) {
                int axis = dev->axmap[i];
                printf("%s=%d ",
                    axis < AXMAP_SIZE ? axnames[axis] : "AXIS?",
                    dev->axis_state[axis]);
            }
            printf("] Buttons[");
            for (int i = 0; i < dev->buttons; i++) {
                int btn = dev->btnmap[i];
                printf("%s=%d ",
                    btn < BTNMAP_SIZE ? btnnames[btn] : "BTN?",
                    dev->button_state[btn]);
            }
            printf("]\n");
        }
        usleep(16000); // ~60 Hz
    }
}
