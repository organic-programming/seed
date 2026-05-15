'use strict';

const { root } = require('./root');

const LogsRequest = root.lookupType('holons.v1.LogsRequest');
const LogEntry = root.lookupType('holons.v1.LogEntry');
const ChainHop = root.lookupType('holons.v1.ChainHop');
const MetricsRequest = root.lookupType('holons.v1.MetricsRequest');
const MetricsSnapshot = root.lookupType('holons.v1.MetricsSnapshot');
const MetricSample = root.lookupType('holons.v1.MetricSample');
const HistogramSample = root.lookupType('holons.v1.HistogramSample');
const Bucket = root.lookupType('holons.v1.Bucket');
const EventsRequest = root.lookupType('holons.v1.EventsRequest');
const EventInfo = root.lookupType('holons.v1.EventInfo');
const LogLevel = root.lookupEnum('holons.v1.LogLevel').values;
const EventType = root.lookupEnum('holons.v1.EventType').values;

function makeSerializer(type) {
    return (value) => Buffer.from(type.encode(type.fromObject(value || {})).finish());
}

function makeDeserializer(type) {
    return (buffer) => type.toObject(type.decode(buffer), {
        longs: String,
        enums: String,
        defaults: true,
        arrays: true,
        objects: true,
        oneofs: true,
    });
}

function method(path, requestType, responseType, responseStream, originalName) {
    return {
        path,
        requestStream: false,
        responseStream,
        requestSerialize: makeSerializer(requestType),
        requestDeserialize: makeDeserializer(requestType),
        responseSerialize: makeSerializer(responseType),
        responseDeserialize: makeDeserializer(responseType),
        originalName,
    };
}

const HOLON_OBSERVABILITY_SERVICE_DEF = {
    Logs: method('/holons.v1.HolonObservability/Logs', LogsRequest, LogEntry, true, 'logs'),
    Metrics: method('/holons.v1.HolonObservability/Metrics', MetricsRequest, MetricsSnapshot, false, 'metrics'),
    Events: method('/holons.v1.HolonObservability/Events', EventsRequest, EventInfo, true, 'events'),
};

module.exports = {
    root,
    LogsRequest,
    LogEntry,
    ChainHop,
    MetricsRequest,
    MetricsSnapshot,
    MetricSample,
    HistogramSample,
    Bucket,
    EventsRequest,
    EventInfo,
    LogLevel,
    EventType,
    HOLON_OBSERVABILITY_SERVICE_DEF,
};
