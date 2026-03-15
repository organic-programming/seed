package org.organicprogramming.hello

import kotlin.test.Test
import kotlin.test.assertEquals

class HelloServiceTest {
    @Test fun greetWithName() {
        assertEquals("Hello, Alice!", HelloService.greet("Alice"))
    }

    @Test fun greetDefault() {
        assertEquals("Hello, World!", HelloService.greet(""))
    }
}
