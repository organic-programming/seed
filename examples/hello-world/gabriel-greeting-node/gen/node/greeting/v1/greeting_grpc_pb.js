// GENERATED CODE -- DO NOT EDIT!

'use strict';
var grpc = require('@grpc/grpc-js');
var v1_greeting_pb = require('../v1/greeting_pb.js');

function serialize_greeting_v1_ListLanguagesRequest(arg) {
  if (!(arg instanceof v1_greeting_pb.ListLanguagesRequest)) {
    throw new Error('Expected argument of type greeting.v1.ListLanguagesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_greeting_v1_ListLanguagesRequest(buffer_arg) {
  return v1_greeting_pb.ListLanguagesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_greeting_v1_ListLanguagesResponse(arg) {
  if (!(arg instanceof v1_greeting_pb.ListLanguagesResponse)) {
    throw new Error('Expected argument of type greeting.v1.ListLanguagesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_greeting_v1_ListLanguagesResponse(buffer_arg) {
  return v1_greeting_pb.ListLanguagesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_greeting_v1_SayHelloRequest(arg) {
  if (!(arg instanceof v1_greeting_pb.SayHelloRequest)) {
    throw new Error('Expected argument of type greeting.v1.SayHelloRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_greeting_v1_SayHelloRequest(buffer_arg) {
  return v1_greeting_pb.SayHelloRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_greeting_v1_SayHelloResponse(arg) {
  if (!(arg instanceof v1_greeting_pb.SayHelloResponse)) {
    throw new Error('Expected argument of type greeting.v1.SayHelloResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_greeting_v1_SayHelloResponse(buffer_arg) {
  return v1_greeting_pb.SayHelloResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// Language-neutral service contract for the Greeting daemon family.
// This file carries NO language-specific options and NO manifest data.
// Each daemon implementation imports it and layers its own metadata on top.
//
var GreetingServiceService = exports.GreetingServiceService = {
  // Returns all available greeting languages.
// @example {}
listLanguages: {
    path: '/greeting.v1.GreetingService/ListLanguages',
    requestStream: false,
    responseStream: false,
    requestType: v1_greeting_pb.ListLanguagesRequest,
    responseType: v1_greeting_pb.ListLanguagesResponse,
    requestSerialize: serialize_greeting_v1_ListLanguagesRequest,
    requestDeserialize: deserialize_greeting_v1_ListLanguagesRequest,
    responseSerialize: serialize_greeting_v1_ListLanguagesResponse,
    responseDeserialize: deserialize_greeting_v1_ListLanguagesResponse,
  },
  // Greets the user in the chosen language.
// @example {"name":"Alice","lang_code":"fr"}
sayHello: {
    path: '/greeting.v1.GreetingService/SayHello',
    requestStream: false,
    responseStream: false,
    requestType: v1_greeting_pb.SayHelloRequest,
    responseType: v1_greeting_pb.SayHelloResponse,
    requestSerialize: serialize_greeting_v1_SayHelloRequest,
    requestDeserialize: deserialize_greeting_v1_SayHelloRequest,
    responseSerialize: serialize_greeting_v1_SayHelloResponse,
    responseDeserialize: deserialize_greeting_v1_SayHelloResponse,
  },
};

exports.GreetingServiceClient = grpc.makeGenericClientConstructor(GreetingServiceService, 'GreetingService');
