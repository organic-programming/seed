'use strict';

const { root } = require('./root');

const DescribeRequest = root.lookupType('holons.v1.DescribeRequest');
const DescribeResponse = root.lookupType('holons.v1.DescribeResponse');
const ServiceDoc = root.lookupType('holons.v1.ServiceDoc');
const MethodDoc = root.lookupType('holons.v1.MethodDoc');
const FieldDoc = root.lookupType('holons.v1.FieldDoc');
const EnumValueDoc = root.lookupType('holons.v1.EnumValueDoc');
const FieldLabel = root.lookupEnum('holons.v1.FieldLabel').values;

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

const HOLON_META_SERVICE_DEF = {
    Describe: {
        path: '/holons.v1.HolonMeta/Describe',
        requestStream: false,
        responseStream: false,
        requestSerialize: makeSerializer(DescribeRequest),
        requestDeserialize: makeDeserializer(DescribeRequest),
        responseSerialize: makeSerializer(DescribeResponse),
        responseDeserialize: makeDeserializer(DescribeResponse),
        originalName: 'describe',
    },
};

module.exports = {
    root,
    DescribeRequest,
    DescribeResponse,
    ServiceDoc,
    MethodDoc,
    FieldDoc,
    EnumValueDoc,
    FieldLabel,
    HOLON_META_SERVICE_DEF,
};
