'use strict';

const { root } = require('./root');

const ListMembersRequest = root.lookupType('holons.v1.ListMembersRequest');
const ListMembersResponse = root.lookupType('holons.v1.ListMembersResponse');
const MemberInfo = root.lookupType('holons.v1.MemberInfo');
const MemberState = root.lookupEnum('holons.v1.MemberState').values;
const MemberStatusRequest = root.lookupType('holons.v1.MemberStatusRequest');
const MemberStatusResponse = root.lookupType('holons.v1.MemberStatusResponse');
const ConnectMemberRequest = root.lookupType('holons.v1.ConnectMemberRequest');
const ConnectMemberResponse = root.lookupType('holons.v1.ConnectMemberResponse');
const DisconnectMemberRequest = root.lookupType('holons.v1.DisconnectMemberRequest');
const DisconnectMemberResponse = root.lookupType('holons.v1.DisconnectMemberResponse');
const TellRequest = root.lookupType('holons.v1.TellRequest');
const TellResponse = root.lookupType('holons.v1.TellResponse');
const TurnOffCoaxRequest = root.lookupType('holons.v1.TurnOffCoaxRequest');
const TurnOffCoaxResponse = root.lookupType('holons.v1.TurnOffCoaxResponse');

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

function unary(path, requestType, responseType, originalName) {
    return {
        path,
        requestStream: false,
        responseStream: false,
        requestSerialize: makeSerializer(requestType),
        requestDeserialize: makeDeserializer(requestType),
        responseSerialize: makeSerializer(responseType),
        responseDeserialize: makeDeserializer(responseType),
        originalName,
    };
}

const COAX_SERVICE_DEF = {
    ListMembers: unary('/holons.v1.CoaxService/ListMembers', ListMembersRequest, ListMembersResponse, 'listMembers'),
    MemberStatus: unary('/holons.v1.CoaxService/MemberStatus', MemberStatusRequest, MemberStatusResponse, 'memberStatus'),
    ConnectMember: unary('/holons.v1.CoaxService/ConnectMember', ConnectMemberRequest, ConnectMemberResponse, 'connectMember'),
    DisconnectMember: unary('/holons.v1.CoaxService/DisconnectMember', DisconnectMemberRequest, DisconnectMemberResponse, 'disconnectMember'),
    Tell: unary('/holons.v1.CoaxService/Tell', TellRequest, TellResponse, 'tell'),
    TurnOffCoax: unary('/holons.v1.CoaxService/TurnOffCoax', TurnOffCoaxRequest, TurnOffCoaxResponse, 'turnOffCoax'),
};

module.exports = {
    root,
    ListMembersRequest,
    ListMembersResponse,
    MemberInfo,
    MemberState,
    MemberStatusRequest,
    MemberStatusResponse,
    ConnectMemberRequest,
    ConnectMemberResponse,
    DisconnectMemberRequest,
    DisconnectMemberResponse,
    TellRequest,
    TellResponse,
    TurnOffCoaxRequest,
    TurnOffCoaxResponse,
    COAX_SERVICE_DEF,
};
