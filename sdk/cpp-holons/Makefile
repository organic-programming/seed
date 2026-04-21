CMAKE ?= cmake
BUILD_DIR ?= build

.PHONY: test clean

test:
	$(CMAKE) -S . -B $(BUILD_DIR)
	$(CMAKE) --build $(BUILD_DIR) --target test_runner
	./$(BUILD_DIR)/test_runner

clean:
	rm -rf $(BUILD_DIR) test_runner
