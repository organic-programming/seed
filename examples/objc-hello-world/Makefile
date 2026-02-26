CC ?= clang
FOUNDATION := -framework Foundation

.PHONY: all test clean

all: hello

hello: hello.m
	$(CC) $(FOUNDATION) hello.m -o hello

test: test_runner
	./test_runner

test_runner: hello.m test_hello.m
	$(CC) $(FOUNDATION) -DTEST_BUILD hello.m test_hello.m -o test_runner

clean:
	rm -f hello test_runner
