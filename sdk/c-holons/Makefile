CC ?= clang
CFLAGS ?= -std=c11 -Wall -Wextra -pedantic -I include
LDFLAGS ?=

ifneq ($(strip $(OP_SDK_C_PATH)),)
  ifeq ($(wildcard $(OP_SDK_C_PATH)/include),)
    $(error OP_SDK_C_PATH=$(OP_SDK_C_PATH) is not a C SDK prebuilt root; run `op sdk install c`)
  endif
  ifeq ($(wildcard $(OP_SDK_C_PATH)/lib),)
    $(error OP_SDK_C_PATH=$(OP_SDK_C_PATH) is not a C SDK prebuilt root; run `op sdk install c`)
  endif
  CFLAGS += -I$(OP_SDK_C_PATH)/include
  LDFLAGS += -L$(OP_SDK_C_PATH)/lib
endif

TEST_BIN := test_runner
SRC := src/holons.c src/discovery_api.c src/observability.c
TEST_SRC := test/holons_test.c
DESCRIBE_HELPER_BIN := describe-static-helper
DESCRIBE_HELPER_SRC := test/describe_static_helper.c test/describe_generated_fixture.c

.PHONY: test clean

test: $(TEST_BIN) $(DESCRIBE_HELPER_BIN)
	./$(TEST_BIN)

$(TEST_BIN): $(SRC) $(TEST_SRC) include/holons/holons.h include/holons/observability.h
	$(CC) $(CFLAGS) $(SRC) $(TEST_SRC) $(LDFLAGS) -o $(TEST_BIN)

$(DESCRIBE_HELPER_BIN): $(SRC) $(DESCRIBE_HELPER_SRC) include/holons/holons.h include/holons/observability.h
	$(CC) $(CFLAGS) $(SRC) $(DESCRIBE_HELPER_SRC) $(LDFLAGS) -o $(DESCRIBE_HELPER_BIN)

clean:
	rm -f $(TEST_BIN) $(DESCRIBE_HELPER_BIN)
