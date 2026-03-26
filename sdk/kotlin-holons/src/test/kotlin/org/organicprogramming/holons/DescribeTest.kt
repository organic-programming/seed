package org.organicprogramming.holons

import holons.v1.Describe as HolonsDescribe
import io.grpc.CallOptions
import io.grpc.ManagedChannelBuilder
import io.grpc.ServerBuilder
import io.grpc.stub.ClientCalls
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.TimeUnit
import kotlin.io.path.createDirectories
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class DescribeTest {
    @Test
    fun buildResponseFromEchoProto() {
        val root = Files.createTempDirectory("kotlin-holons-describe")
        try {
            writeEchoHolon(root)

            val response = Describe.buildResponse(root.resolve("protos"))
            assertEquals("Echo", response.manifest.identity.givenName)
            assertEquals("Server", response.manifest.identity.familyName)
            assertEquals("Reply precisely.", response.manifest.identity.motto)
            assertEquals(1, response.servicesCount)

            val service = response.servicesList.single()
            assertEquals("echo.v1.Echo", service.name)
            assertEquals("Echo echoes request payloads for documentation tests.", service.description)

            val method = service.methodsList.single()
            assertEquals("Ping", method.name)
            assertEquals("echo.v1.PingRequest", method.inputType)
            assertEquals("echo.v1.PingResponse", method.outputType)
            assertEquals("""{"message":"hello","sdk":"go-holons"}""", method.exampleInput)

            val field = method.inputFieldsList.first()
            assertEquals("message", field.name)
            assertEquals("string", field.type)
            assertEquals(1, field.number)
            assertEquals("Message to echo back.", field.description)
            assertEquals(HolonsDescribe.FieldLabel.FIELD_LABEL_OPTIONAL, field.label)
            assertTrue(field.required)
            assertEquals(""""hello"""", field.example)
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun registersWorkingDescribeRpc() {
        val root = Files.createTempDirectory("kotlin-holons-describe-rpc")
        writeEchoHolon(root)
        Describe.useStaticResponse(Describe.buildResponse(root.resolve("protos")))

        val server = ServerBuilder.forPort(0)
            .addService(Describe.serviceDefinition())
            .build()
            .start()
        val channel = ManagedChannelBuilder
            .forAddress("127.0.0.1", server.port)
            .usePlaintext()
            .build()

        try {
            val response = ClientCalls.blockingUnaryCall(
                channel,
                Describe.describeMethod(),
                CallOptions.DEFAULT,
                HolonsDescribe.DescribeRequest.getDefaultInstance(),
            )

            assertEquals("Echo", response.manifest.identity.givenName)
            assertEquals(1, response.servicesCount)
            assertEquals("echo.v1.Echo", response.servicesList.single().name)
            assertEquals("Ping", response.servicesList.single().methodsList.single().name)
        } finally {
            Describe.useStaticResponse(null)
            channel.shutdownNow()
            channel.awaitTermination(5, TimeUnit.SECONDS)
            server.shutdownNow()
            server.awaitTermination(5, TimeUnit.SECONDS)
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun serviceDefinitionRequiresRegisteredStaticResponse() {
        Describe.useStaticResponse(null)

        val error = assertFailsWith<IllegalStateException> {
            Describe.serviceDefinition()
        }

        assertEquals(Describe.NO_INCODE_DESCRIPTION_MESSAGE, error.message)
    }

    @Test
    fun handlesMissingProtoDirectory() {
        val root = Files.createTempDirectory("kotlin-holons-describe-empty")
        try {
            Files.writeString(
                root.resolve("holon.proto"),
                """
                syntax = "proto3";
                package test.v1;

                option (holons.v1.manifest) = {
                  identity: {
                    given_name: "Silent"
                    family_name: "Holon"
                    motto: "Quietly available."
                  }
                };
                """.trimIndent(),
            )

            val response = Describe.buildResponse(root.resolve("protos"))
            assertEquals("Silent", response.manifest.identity.givenName)
            assertEquals("Holon", response.manifest.identity.familyName)
            assertEquals("Quietly available.", response.manifest.identity.motto)
            assertEquals(0, response.servicesCount)
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    private fun writeEchoHolon(root: Path) {
        val protoDir = root.resolve("protos/echo/v1")
        protoDir.createDirectories()

        Files.writeString(
            root.resolve("holon.proto"),
            """
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                given_name: "Echo"
                family_name: "Server"
                motto: "Reply precisely."
              }
            };
            """.trimIndent(),
        )

        Files.writeString(
            protoDir.resolve("echo.proto"),
            """
            syntax = "proto3";
            package echo.v1;

            // Echo echoes request payloads for documentation tests.
            service Echo {
              // Ping echoes the inbound message.
              // @example {"message":"hello","sdk":"go-holons"}
              rpc Ping(PingRequest) returns (PingResponse);
            }

            message PingRequest {
              // Message to echo back.
              // @required
              // @example "hello"
              string message = 1;

              // SDK marker included in the response.
              // @example "go-holons"
              string sdk = 2;
            }

            message PingResponse {
              // Echoed message.
              string message = 1;

              // SDK marker from the server.
              string sdk = 2;
            }
            """.trimIndent(),
        )
    }
}
