package org.organicprogramming.gabriel.greeting.kotlincompose.rpc

import io.grpc.StatusRuntimeException
import io.grpc.stub.StreamObserver

internal inline fun <T> respond(
    responseObserver: StreamObserver<T>,
    block: () -> T,
) {
    try {
        responseObserver.onNext(block())
        responseObserver.onCompleted()
    } catch (error: StatusRuntimeException) {
        responseObserver.onError(error)
    } catch (error: Throwable) {
        responseObserver.onError(error)
    }
}
