CC ?= cc
CFLAGS ?= -std=c11 -Wall -Wextra -pedantic
INCLUDES := -I ../../../sdk/c-holons/include
SDK_SRC := ../../../sdk/c-holons/src/holons.c

APP := hello
TEST_BIN := test_runner

.PHONY: all test clean

all: $(APP)

$(APP): hello.c main.c hello.h $(SDK_SRC)
	$(CC) $(CFLAGS) $(INCLUDES) hello.c main.c $(SDK_SRC) -o $(APP)

$(TEST_BIN): hello.c test_hello.c hello.h $(SDK_SRC)
	$(CC) $(CFLAGS) $(INCLUDES) hello.c test_hello.c $(SDK_SRC) -o $(TEST_BIN)

test: $(TEST_BIN)
	./$(TEST_BIN)

clean:
	rm -f $(APP) $(TEST_BIN)
