package org.organicprogramming.hello;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class HelloServiceTest {
    @Test
    void greetWithName() {
        assertEquals("Hello, Alice!", HelloService.greet("Alice"));
    }

    @Test
    void greetDefault() {
        assertEquals("Hello, World!", HelloService.greet(""));
    }

    @Test
    void greetNull() {
        assertEquals("Hello, World!", HelloService.greet(null));
    }
}
