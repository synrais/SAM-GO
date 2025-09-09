#ifndef __AXBTNMAP_H__
#define __AXBTNMAP_H__

#include <stdint.h>
#include <linux/input.h>

#define KEY_MAX_LARGE 0x2FF
#define KEY_MAX_SMALL 0x1FF
#define AXMAP_SIZE (ABS_MAX + 1)
#define BTNMAP_SIZE (KEY_MAX_LARGE - BTN_MISC + 1)

extern const char *axnames[AXMAP_SIZE];
extern const char *btnnames[BTNMAP_SIZE];

int getaxmap(int fd, uint8_t *axmap);
int setaxmap(int fd, uint8_t *axmap);
int getbtnmap(int fd, uint16_t *btnmap);
int setbtnmap(int fd, uint16_t *btnmap);

#endif
