CC ?= clang
FOUNDATION := -framework Foundation
SDK_ROOT := ../../sdk/objc-holons
SDK_INCLUDE := -I $(SDK_ROOT)/include
SDK_SRC := $(SDK_ROOT)/src/Holons.m $(SDK_ROOT)/src/HolonMeta.m
COMMON_FLAGS := $(FOUNDATION) $(SDK_INCLUDE)

.PHONY: all test clean

all: hello connect_example

hello: hello.m $(SDK_SRC)
	$(CC) $(COMMON_FLAGS) $(SDK_SRC) hello.m -o hello

connect_example: connect_example.m $(SDK_SRC)
	$(CC) $(COMMON_FLAGS) $(SDK_SRC) connect_example.m -o connect_example

test: test_runner
	./test_runner

test_runner: hello.m test_hello.m $(SDK_SRC)
	$(CC) $(COMMON_FLAGS) -DTEST_BUILD $(SDK_SRC) hello.m test_hello.m -o test_runner

clean:
	rm -f hello connect_example test_runner
