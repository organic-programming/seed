CC ?= clang
CFLAGS ?= -std=c11 -Wall -Wextra -pedantic -I include

TEST_BIN := test_runner
SRC := src/holons.c
TEST_SRC := test/holons_test.c
DESCRIBE_HELPER_BIN := describe-static-helper
DESCRIBE_HELPER_SRC := test/describe_static_helper.c test/describe_generated_fixture.c

.PHONY: test clean

test: $(TEST_BIN) $(DESCRIBE_HELPER_BIN)
	./$(TEST_BIN)

$(TEST_BIN): $(SRC) $(TEST_SRC) include/holons/holons.h
	$(CC) $(CFLAGS) $(SRC) $(TEST_SRC) -o $(TEST_BIN)

$(DESCRIBE_HELPER_BIN): $(SRC) $(DESCRIBE_HELPER_SRC) include/holons/holons.h
	$(CC) $(CFLAGS) $(SRC) $(DESCRIBE_HELPER_SRC) -o $(DESCRIBE_HELPER_BIN)

clean:
	rm -f $(TEST_BIN) $(DESCRIBE_HELPER_BIN)
