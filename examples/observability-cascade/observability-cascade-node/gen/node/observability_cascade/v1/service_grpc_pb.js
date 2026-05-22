// GENERATED CODE -- DO NOT EDIT!

'use strict';
var grpc = require('@grpc/grpc-js');
var observability_cascade_v1_service_pb = require('../../observability_cascade/v1/service_pb.js');

function serialize_observability_cascade_v1_CascadeReport(arg) {
  if (!(arg instanceof observability_cascade_v1_service_pb.CascadeReport)) {
    throw new Error('Expected argument of type observability_cascade.v1.CascadeReport');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_observability_cascade_v1_CascadeReport(buffer_arg) {
  return observability_cascade_v1_service_pb.CascadeReport.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_observability_cascade_v1_MultiPatternReport(arg) {
  if (!(arg instanceof observability_cascade_v1_service_pb.MultiPatternReport)) {
    throw new Error('Expected argument of type observability_cascade.v1.MultiPatternReport');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_observability_cascade_v1_MultiPatternReport(buffer_arg) {
  return observability_cascade_v1_service_pb.MultiPatternReport.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_observability_cascade_v1_RunRequest(arg) {
  if (!(arg instanceof observability_cascade_v1_service_pb.RunRequest)) {
    throw new Error('Expected argument of type observability_cascade.v1.RunRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_observability_cascade_v1_RunRequest(buffer_arg) {
  return observability_cascade_v1_service_pb.RunRequest.deserializeBinary(new Uint8Array(buffer_arg));
}


var ObservabilityCascadeServiceService = exports.ObservabilityCascadeServiceService = {
  // Run the default 4-deep chain in this composite's own language.
  // @example {}
  runDefault: {
    path: '/observability_cascade.v1.ObservabilityCascadeService/RunDefault',
    requestStream: false,
    responseStream: false,
    requestType: observability_cascade_v1_service_pb.RunRequest,
    responseType: observability_cascade_v1_service_pb.CascadeReport,
    requestSerialize: serialize_observability_cascade_v1_RunRequest,
    requestDeserialize: deserialize_observability_cascade_v1_RunRequest,
    responseSerialize: serialize_observability_cascade_v1_CascadeReport,
    responseDeserialize: deserialize_observability_cascade_v1_CascadeReport,
  },
  // Run with long-lived Follow:true streams.
  // @example {}
  runLiveStream: {
    path: '/observability_cascade.v1.ObservabilityCascadeService/RunLiveStream',
    requestStream: false,
    responseStream: false,
    requestType: observability_cascade_v1_service_pb.RunRequest,
    responseType: observability_cascade_v1_service_pb.CascadeReport,
    requestSerialize: serialize_observability_cascade_v1_RunRequest,
    requestDeserialize: deserialize_observability_cascade_v1_RunRequest,
    responseSerialize: serialize_observability_cascade_v1_CascadeReport,
    responseDeserialize: deserialize_observability_cascade_v1_CascadeReport,
  },
  // Run the full alter-language pattern matrix (3 patterns x 12 ticks = 36 ticks).
  // @example {}
  runMultiPattern: {
    path: '/observability_cascade.v1.ObservabilityCascadeService/RunMultiPattern',
    requestStream: false,
    responseStream: false,
    requestType: observability_cascade_v1_service_pb.RunRequest,
    responseType: observability_cascade_v1_service_pb.MultiPatternReport,
    requestSerialize: serialize_observability_cascade_v1_RunRequest,
    requestDeserialize: deserialize_observability_cascade_v1_RunRequest,
    responseSerialize: serialize_observability_cascade_v1_MultiPatternReport,
    responseDeserialize: deserialize_observability_cascade_v1_MultiPatternReport,
  },
};

exports.ObservabilityCascadeServiceClient = grpc.makeGenericClientConstructor(ObservabilityCascadeServiceService);
