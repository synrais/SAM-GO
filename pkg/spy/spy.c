#include <linux/hidraw.h>
#include <sys/ioctl.h>
#include <poll.h>

#define REPORT_SIZE 64

typedef struct {
    int fd;
    char path[64];
    int is_js;
    char name[128];
    unsigned char axes;
    unsigned char buttons;
    uint8_t axmap[AXMAP_SIZE];
    uint16_t btnmap[BTNMAP_SIZE];
    int axis_state[AXMAP_SIZE];
    int button_state[BTNMAP_SIZE];
} input_dev;

static input_dev devices[MAX_DEVICES];
static int dev_count = 0;

void spy_scan_devices() {
    char path[64];
    // js devices
    for (int i = 0; i < MAX_DEVICES; i++) {
        snprintf(path, sizeof(path), "/dev/input/js%d", i);
        int fd = open(path, O_RDONLY | O_NONBLOCK);
        if (fd < 0) continue;
        input_dev *dev = &devices[dev_count++];
        memset(dev, 0, sizeof(*dev));
        dev->fd = fd;
        dev->is_js = 1;
        strncpy(dev->path, path, sizeof(dev->path));

        ioctl(fd, JSIOCGNAME(sizeof(dev->name)), dev->name);
        ioctl(fd, JSIOCGAXES, &dev->axes);
        ioctl(fd, JSIOCGBUTTONS, &dev->buttons);
        getaxmap(fd, dev->axmap);
        getbtnmap(fd, dev->btnmap);

        printf("Monitoring %s (%s)\n", path, dev->name);
        printf("  Axes: %d  Buttons: %d\n", dev->axes, dev->buttons);
    }

    // hidraw devices
    for (int i = 0; i < MAX_DEVICES; i++) {
        snprintf(path, sizeof(path), "/dev/hidraw%d", i);
        int fd = open(path, O_RDONLY | O_NONBLOCK);
        if (fd < 0) continue;
        input_dev *dev = &devices[dev_count++];
        memset(dev, 0, sizeof(*dev));
        dev->fd = fd;
        dev->is_js = 0;
        strncpy(dev->path, path, sizeof(dev->path));
        snprintf(dev->name, sizeof(dev->name), "hidraw%d", i);
        printf("Monitoring %s (hidraw)\n", path);
    }
}

void spy_loop() {
    struct pollfd fds[MAX_DEVICES];
    struct js_event e;
    unsigned char buf[REPORT_SIZE];

    for (;;) {
        for (int i = 0; i < dev_count; i++) {
            fds[i].fd = devices[i].fd;
            fds[i].events = POLLIN;
        }
        int ret = poll(fds, dev_count, 50);
        if (ret <= 0) continue;

        for (int i = 0; i < dev_count; i++) {
            if (!(fds[i].revents & POLLIN)) continue;
            input_dev *dev = &devices[i];

            if (dev->is_js) {
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
                printf("[%s] Axes[", dev->name);
                for (int j = 0; j < dev->axes; j++) {
                    int axis = dev->axmap[j];
                    printf("%s=%d ",
                        axis < AXMAP_SIZE ? axnames[axis] : "AXIS?",
                        dev->axis_state[axis]);
                }
                printf("] Buttons[");
                for (int j = 0; j < dev->buttons; j++) {
                    int btn = dev->btnmap[j];
                    printf("%s=%d ",
                        btn < BTNMAP_SIZE ? btnnames[btn] : "BTN?",
                        dev->button_state[btn]);
                }
                printf("]\n");
            } else {
                int n = read(dev->fd, buf, sizeof(buf));
                if (n > 0) {
                    printf("[%s] HID report:", dev->name);
                    for (int j = 0; j < n; j++)
                        printf(" %02x", buf[j]);
                    printf("\n");
                }
            }
        }
    }
}
