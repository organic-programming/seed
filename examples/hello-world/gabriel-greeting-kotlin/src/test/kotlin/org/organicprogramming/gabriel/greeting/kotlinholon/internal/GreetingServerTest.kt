package org.organicprogramming.gabriel.greeting.kotlinholon.internal

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpcKt
import io.grpc.inprocess.InProcessChannelBuilder
import io.grpc.inprocess.InProcessServerBuilder
import kotlinx.coroutines.test.runTest
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse

class GreetingServerTest {
    @Test
    fun listLanguagesReturnsAllLanguages() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance())
                assertEquals(56, response.languagesCount)
                assertFalse(response.languagesList.any { it.code.isEmpty() || it.name.isEmpty() || it.native.isEmpty() })
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }

    @Test
    fun sayHelloUsesRequestedLanguage() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.sayHello(
                    Greeting.SayHelloRequest.newBuilder()
                        .setName("Bob")
                        .setLangCode("fr")
                        .build(),
                )
                assertEquals("Bonjour Bob", response.greeting)
                assertEquals("French", response.language)
                assertEquals("fr", response.langCode)
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }

    @Test
    fun sayHelloUsesLocalizedDefaultName() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.sayHello(
                    Greeting.SayHelloRequest.newBuilder()
                        .setLangCode("fr")
                        .build(),
                )
                assertEquals("Bonjour Marie", response.greeting)
                assertEquals("fr", response.langCode)
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }

    @Test
    fun sayHelloFallsBackToEnglish() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.sayHello(
                    Greeting.SayHelloRequest.newBuilder()
                        .setName("Bob")
                        .setLangCode("xx")
                        .build(),
                )
                assertEquals("Hello Bob", response.greeting)
                assertEquals("en", response.langCode)
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }
}
