// GENERATED CODE -- DO NOT EDIT!

'use strict';
var grpc = require('@grpc/grpc-js');
var relay_v1_relay_pb = require('../../relay/v1/relay_pb.js');

function serialize_relay_v1_TickRequest(arg) {
  if (!(arg instanceof relay_v1_relay_pb.TickRequest)) {
    throw new Error('Expected argument of type relay.v1.TickRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_relay_v1_TickRequest(buffer_arg) {
  return relay_v1_relay_pb.TickRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_relay_v1_TickResponse(arg) {
  if (!(arg instanceof relay_v1_relay_pb.TickResponse)) {
    throw new Error('Expected argument of type relay.v1.TickResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_relay_v1_TickResponse(buffer_arg) {
  return relay_v1_relay_pb.TickResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var RelayServiceService = exports.RelayServiceService = {
  // Tick: emit one log + increment one metric counter at the receiver.
  // Used to test cross-holon observability relay: send a Tick to a leaf
  // holon and verify the log propagates up the MemberEndpoints chain.
  // Metrics are NOT relayed by the SDK - they are exposed locally and
  // verified at each node directly.
  tick: {
    path: '/relay.v1.RelayService/Tick',
    requestStream: false,
    responseStream: false,
    requestType: relay_v1_relay_pb.TickRequest,
    responseType: relay_v1_relay_pb.TickResponse,
    requestSerialize: serialize_relay_v1_TickRequest,
    requestDeserialize: deserialize_relay_v1_TickRequest,
    responseSerialize: serialize_relay_v1_TickResponse,
    responseDeserialize: deserialize_relay_v1_TickResponse,
  },
};

exports.RelayServiceClient = grpc.makeGenericClientConstructor(RelayServiceService);
