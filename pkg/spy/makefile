# pkg/spy/Makefile

# Cross-compiler (make sure you have arm-linux-gnueabihf-gcc installed)
CC      := arm-linux-gnueabihf-gcc
AR      := arm-linux-gnueabihf-ar
CFLAGS  := -O2 -Wall -static
SRC     := spy.c
OBJ     := $(SRC:.c=.o)
LIB     := libspy.a

all: $(LIB)

$(OBJ): $(SRC) spy.h
	$(CC) $(CFLAGS) -c $< -o $@

$(LIB): $(OBJ)
	$(AR) rcs $@ $^

clean:
	rm -f $(OBJ) $(LIB)
