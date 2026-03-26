import { root } from "./root.mjs";

export const ListMembersRequest = root.lookupType("holons.v1.ListMembersRequest");
export const ListMembersResponse = root.lookupType("holons.v1.ListMembersResponse");
export const MemberInfo = root.lookupType("holons.v1.MemberInfo");
export const MemberState = root.lookupEnum("holons.v1.MemberState").values;
export const MemberStatusRequest = root.lookupType("holons.v1.MemberStatusRequest");
export const MemberStatusResponse = root.lookupType("holons.v1.MemberStatusResponse");
export const ConnectMemberRequest = root.lookupType("holons.v1.ConnectMemberRequest");
export const ConnectMemberResponse = root.lookupType("holons.v1.ConnectMemberResponse");
export const DisconnectMemberRequest = root.lookupType("holons.v1.DisconnectMemberRequest");
export const DisconnectMemberResponse = root.lookupType("holons.v1.DisconnectMemberResponse");
export const TellRequest = root.lookupType("holons.v1.TellRequest");
export const TellResponse = root.lookupType("holons.v1.TellResponse");
export const TurnOffCoaxRequest = root.lookupType("holons.v1.TurnOffCoaxRequest");
export const TurnOffCoaxResponse = root.lookupType("holons.v1.TurnOffCoaxResponse");
