#include "axbtnmap.h"

const char *axnames[AXMAP_SIZE] = {
    [ABS_X] = "ABS_X", [ABS_Y] = "ABS_Y", [ABS_Z] = "ABS_Z",
    [ABS_RX] = "ABS_RX", [ABS_RY] = "ABS_RY", [ABS_RZ] = "ABS_RZ",
    [ABS_HAT0X] = "ABS_HAT0X", [ABS_HAT0Y] = "ABS_HAT0Y",
    [ABS_HAT1X] = "ABS_HAT1X", [ABS_HAT1Y] = "ABS_HAT1Y",
    // add more if needed
};

const char *btnnames[BTNMAP_SIZE] = {
    [BTN_SOUTH] = "BTN_SOUTH", [BTN_EAST] = "BTN_EAST",
    [BTN_NORTH] = "BTN_NORTH", [BTN_WEST] = "BTN_WEST",
    [BTN_TL] = "BTN_TL", [BTN_TR] = "BTN_TR",
    [BTN_SELECT] = "BTN_SELECT", [BTN_START] = "BTN_START",
    [BTN_MODE] = "BTN_MODE", [BTN_THUMBL] = "BTN_THUMBL",
    [BTN_THUMBR] = "BTN_THUMBR",
    // add more if needed
};
