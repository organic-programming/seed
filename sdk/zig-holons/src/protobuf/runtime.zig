const std = @import("std");

pub const c = @cImport({
    @cInclude("protobuf-c/protobuf-c.h");
    @cInclude("holons/v1/describe.pb-c.h");
    @cInclude("holons/v1/observability.pb-c.h");
    @cInclude("v1/greeting.pb-c.h");
});

pub const RuntimeVersion = struct {
    text: [*:0]const u8,
    number: c_uint,
};

pub fn version() RuntimeVersion {
    return .{
        .text = c.protobuf_c_version(),
        .number = c.protobuf_c_version_number(),
    };
}

pub const DescribeResponse = struct {
    raw: *c.Holons__V1__DescribeResponse,

    pub fn deinit(self: *DescribeResponse) void {
        c.holons__v1__describe_response__free_unpacked(self.raw, null);
    }

    pub fn familyName(self: DescribeResponse) []const u8 {
        const manifest = self.raw.*.manifest orelse return "";
        const identity = manifest.*.identity orelse return "";
        return cstr(identity.*.family_name);
    }

    pub fn uuid(self: DescribeResponse) []const u8 {
        const manifest = self.raw.*.manifest orelse return "";
        const identity = manifest.*.identity orelse return "";
        return cstr(identity.*.uuid);
    }

    pub fn serviceCount(self: DescribeResponse) usize {
        return self.raw.*.n_services;
    }
};

pub const SayHelloResponse = struct {
    raw: *c.Greeting__V1__SayHelloResponse,

    pub fn deinit(self: *SayHelloResponse) void {
        c.greeting__v1__say_hello_response__free_unpacked(self.raw, null);
    }

    pub fn greeting(self: SayHelloResponse) []const u8 {
        return cstr(self.raw.*.greeting);
    }

    pub fn language(self: SayHelloResponse) []const u8 {
        return cstr(self.raw.*.language);
    }

    pub fn langCode(self: SayHelloResponse) []const u8 {
        return cstr(self.raw.*.lang_code);
    }
};

pub const SayHelloRequest = struct {
    raw: *c.Greeting__V1__SayHelloRequest,

    pub fn deinit(self: *SayHelloRequest) void {
        c.greeting__v1__say_hello_request__free_unpacked(self.raw, null);
    }

    pub fn name(self: SayHelloRequest) []const u8 {
        return cstr(self.raw.*.name);
    }

    pub fn langCode(self: SayHelloRequest) []const u8 {
        return cstr(self.raw.*.lang_code);
    }
};

pub const ListLanguagesResponse = struct {
    raw: *c.Greeting__V1__ListLanguagesResponse,

    pub fn deinit(self: *ListLanguagesResponse) void {
        c.greeting__v1__list_languages_response__free_unpacked(self.raw, null);
    }

    pub fn len(self: ListLanguagesResponse) usize {
        return self.raw.*.n_languages;
    }
};

pub const LogsRequestOptions = struct {
    follow: bool = false,
};

pub const EventsRequestOptions = struct {
    follow: bool = false,
    types: []const i32 = &.{},
};

pub const ObservabilityLogRecord = struct {
    raw: *c.Holons__V1__LogRecord,

    pub fn deinit(self: *ObservabilityLogRecord, allocator: std.mem.Allocator) void {
        _ = allocator;
        c.holons__v1__log_record__free_unpacked(self.raw, null);
    }

    pub fn message(self: ObservabilityLogRecord) []const u8 {
        return anyValueString(self.raw.*.body) orelse "";
    }

    pub fn slug(self: ObservabilityLogRecord) []const u8 {
        return logRecordAttributeString(self.raw, "holons.slug") orelse "";
    }

    pub fn eventName(self: ObservabilityLogRecord) []const u8 {
        const explicit = cstr(self.raw.*.event_name);
        return if (explicit.len == 0) anyValueString(self.raw.*.body) orelse "" else explicit;
    }

    pub fn severityNumber(self: ObservabilityLogRecord) i32 {
        return @intCast(self.raw.*.severity_number);
    }

    pub fn attributeString(self: ObservabilityLogRecord, key: []const u8) ?[]const u8 {
        return logRecordAttributeString(self.raw, key);
    }

    pub fn attributeInt(self: ObservabilityLogRecord, key: []const u8) ?i64 {
        for (self.raw.*.attributes[0..self.raw.*.n_attributes]) |attr| {
            if (std.mem.eql(u8, cstr(attr.*.key), key) and attr.*.value != null and
                attr.*.value.*.value_case == c.HOLONS__V1__ANY_VALUE__VALUE_INT_VALUE)
            {
                return attr.*.value.*.unnamed_0.int_value;
            }
        }
        return null;
    }
};

pub const ObservabilityMetric = struct {
    raw: *c.Holons__V1__Metric,

    pub fn deinit(self: *ObservabilityMetric) void {
        c.holons__v1__metric__free_unpacked(self.raw, null);
    }

    pub fn name(self: ObservabilityMetric) []const u8 {
        return cstr(self.raw.*.name);
    }

    pub fn slug(self: ObservabilityMetric) []const u8 {
        return metricAttributeString(self.raw, "holons.slug") orelse "";
    }

    pub fn instanceUid(self: ObservabilityMetric) []const u8 {
        return metricAttributeString(self.raw, "holons.instance_uid") orelse "";
    }

    pub fn sumValue(self: ObservabilityMetric) ?i64 {
        if (self.raw.*.data_case != c.HOLONS__V1__METRIC__DATA_SUM) return null;
        const sum = self.raw.*.unnamed_0.sum orelse return null;
        if (sum.*.n_data_points == 0) return null;
        const point = sum.*.data_points[0];
        if (point.*.value_case != c.HOLONS__V1__NUMBER_DATA_POINT__VALUE_AS_INT) return null;
        return point.*.unnamed_0.as_int;
    }

    pub fn dataCase(self: ObservabilityMetric) i32 {
        return @intCast(self.raw.*.data_case);
    }

    pub fn sumIsMonotonic(self: ObservabilityMetric) bool {
        if (self.raw.*.data_case != c.HOLONS__V1__METRIC__DATA_SUM) return false;
        const sum = self.raw.*.unnamed_0.sum orelse return false;
        return sum.*.is_monotonic != 0;
    }
};

pub const ObservabilityEventRecord = struct {
    raw: *c.Holons__V1__LogRecord,

    pub fn deinit(self: *ObservabilityEventRecord, allocator: std.mem.Allocator) void {
        _ = allocator;
        c.holons__v1__log_record__free_unpacked(self.raw, null);
    }

    pub fn eventName(self: ObservabilityEventRecord) []const u8 {
        const explicit = cstr(self.raw.*.event_name);
        return if (explicit.len == 0) anyValueString(self.raw.*.body) orelse "" else explicit;
    }

    pub fn slug(self: ObservabilityEventRecord) []const u8 {
        return logRecordAttributeString(self.raw, "holons.slug") orelse "";
    }

    pub fn instanceUid(self: ObservabilityEventRecord) []const u8 {
        return logRecordAttributeString(self.raw, "holons.instance_uid") orelse "";
    }

    pub fn chainLen(self: ObservabilityEventRecord) usize {
        return self.raw.*.n_chain;
    }
};

pub const LanguageValue = struct {
    code: []const u8,
    name: []const u8,
    native: []const u8,
};

pub fn packDescribeRequest(allocator: std.mem.Allocator) ![]u8 {
    var request: c.Holons__V1__DescribeRequest = undefined;
    c.holons__v1__describe_request__init(&request);
    return packMessage(
        allocator,
        &request.base,
        c.holons__v1__describe_request__get_packed_size(&request),
    );
}

pub fn packDescribeResponse(allocator: std.mem.Allocator, response: *c.Holons__V1__DescribeResponse) ![]u8 {
    return packMessage(
        allocator,
        &response.base,
        c.holons__v1__describe_response__get_packed_size(response),
    );
}

pub fn unpackDescribeResponse(bytes: []const u8) !DescribeResponse {
    const raw = c.holons__v1__describe_response__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeDescribeResponseFailed;
    return .{ .raw = raw };
}

pub fn unpackSayHelloRequest(bytes: []const u8) !SayHelloRequest {
    const raw = c.greeting__v1__say_hello_request__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeSayHelloRequestFailed;
    return .{ .raw = raw };
}

pub fn packSayHelloRequest(allocator: std.mem.Allocator, name: []const u8, lang_code: []const u8) ![]u8 {
    const name_z = try allocator.dupeZ(u8, name);
    defer allocator.free(name_z);
    const lang_code_z = try allocator.dupeZ(u8, lang_code);
    defer allocator.free(lang_code_z);

    var request: c.Greeting__V1__SayHelloRequest = undefined;
    c.greeting__v1__say_hello_request__init(&request);
    request.name = name_z.ptr;
    request.lang_code = lang_code_z.ptr;
    return packMessage(
        allocator,
        &request.base,
        c.greeting__v1__say_hello_request__get_packed_size(&request),
    );
}

pub fn packSayHelloResponse(
    allocator: std.mem.Allocator,
    greeting: []const u8,
    language: []const u8,
    lang_code: []const u8,
) ![]u8 {
    const greeting_z = try allocator.dupeZ(u8, greeting);
    defer allocator.free(greeting_z);
    const language_z = try allocator.dupeZ(u8, language);
    defer allocator.free(language_z);
    const lang_code_z = try allocator.dupeZ(u8, lang_code);
    defer allocator.free(lang_code_z);

    var response: c.Greeting__V1__SayHelloResponse = undefined;
    c.greeting__v1__say_hello_response__init(&response);
    response.greeting = greeting_z.ptr;
    response.language = language_z.ptr;
    response.lang_code = lang_code_z.ptr;
    return packMessage(
        allocator,
        &response.base,
        c.greeting__v1__say_hello_response__get_packed_size(&response),
    );
}

pub fn unpackSayHelloResponse(bytes: []const u8) !SayHelloResponse {
    const raw = c.greeting__v1__say_hello_response__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeSayHelloResponseFailed;
    return .{ .raw = raw };
}

pub fn unpackListLanguagesRequest(bytes: []const u8) !void {
    const raw = c.greeting__v1__list_languages_request__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeListLanguagesRequestFailed;
    c.greeting__v1__list_languages_request__free_unpacked(raw, null);
}

pub fn packListLanguagesRequest(allocator: std.mem.Allocator) ![]u8 {
    var request: c.Greeting__V1__ListLanguagesRequest = undefined;
    c.greeting__v1__list_languages_request__init(&request);
    return packMessage(
        allocator,
        &request.base,
        c.greeting__v1__list_languages_request__get_packed_size(&request),
    );
}

pub fn packLogsRequest(allocator: std.mem.Allocator, options: LogsRequestOptions) ![]u8 {
    var request: c.Holons__V1__LogsRequest = undefined;
    c.holons__v1__logs_request__init(&request);
    request.follow = @intFromBool(options.follow);
    return packMessage(
        allocator,
        &request.base,
        c.holons__v1__logs_request__get_packed_size(&request),
    );
}

pub fn unpackLogRecord(bytes: []const u8) !ObservabilityLogRecord {
    const raw = c.holons__v1__log_record__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeLogRecordFailed;
    return .{ .raw = raw };
}

pub fn packMetricsRequest(allocator: std.mem.Allocator) ![]u8 {
    var request: c.Holons__V1__MetricsRequest = undefined;
    c.holons__v1__metrics_request__init(&request);
    return packMessage(
        allocator,
        &request.base,
        c.holons__v1__metrics_request__get_packed_size(&request),
    );
}

pub fn unpackMetric(bytes: []const u8) !ObservabilityMetric {
    const raw = c.holons__v1__metric__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeMetricFailed;
    return .{ .raw = raw };
}

pub fn packEventsRequest(allocator: std.mem.Allocator, options: EventsRequestOptions) ![]u8 {
    var request: c.Holons__V1__EventsRequest = undefined;
    c.holons__v1__events_request__init(&request);
    request.follow = @intFromBool(options.follow);
    var event_names = try allocator.alloc([*c]u8, options.types.len);
    defer allocator.free(event_names);
    var owned_names: std.ArrayList([]u8) = .empty;
    defer {
        for (owned_names.items) |item| allocator.free(item);
        owned_names.deinit(allocator);
    }
    for (options.types, 0..) |event_type, index| {
        const name = try allocator.dupeZ(u8, eventNameFromLegacy(event_type));
        try owned_names.append(allocator, name);
        event_names[index] = name.ptr;
    }
    request.n_event_names = options.types.len;
    request.event_names = if (event_names.len == 0) null else event_names.ptr;
    return packMessage(
        allocator,
        &request.base,
        c.holons__v1__events_request__get_packed_size(&request),
    );
}

pub fn unpackEventRecord(bytes: []const u8) !ObservabilityEventRecord {
    const raw = c.holons__v1__log_record__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeEventRecordFailed;
    return .{ .raw = raw };
}

pub fn packListLanguagesResponse(allocator: std.mem.Allocator, languages: []const LanguageValue) ![]u8 {
    var language_messages = try allocator.alloc(c.Greeting__V1__Language, languages.len);
    defer allocator.free(language_messages);
    var language_ptrs = try allocator.alloc([*c]c.Greeting__V1__Language, languages.len);
    defer allocator.free(language_ptrs);

    var owned_strings: std.ArrayList([]u8) = .empty;
    defer {
        for (owned_strings.items) |item| allocator.free(item);
        owned_strings.deinit(allocator);
    }

    for (languages, 0..) |language, index| {
        c.greeting__v1__language__init(&language_messages[index]);
        const code_z = try allocator.dupeZ(u8, language.code);
        errdefer allocator.free(code_z);
        try owned_strings.append(allocator, code_z);
        const name_z = try allocator.dupeZ(u8, language.name);
        errdefer allocator.free(name_z);
        try owned_strings.append(allocator, name_z);
        const native_z = try allocator.dupeZ(u8, language.native);
        errdefer allocator.free(native_z);
        try owned_strings.append(allocator, native_z);
        language_messages[index].code = code_z.ptr;
        language_messages[index].name = name_z.ptr;
        language_messages[index].native = native_z.ptr;
        language_ptrs[index] = &language_messages[index];
    }

    var response: c.Greeting__V1__ListLanguagesResponse = undefined;
    c.greeting__v1__list_languages_response__init(&response);
    response.n_languages = languages.len;
    response.languages = language_ptrs.ptr;
    return packMessage(
        allocator,
        &response.base,
        c.greeting__v1__list_languages_response__get_packed_size(&response),
    );
}

pub fn unpackListLanguagesResponse(bytes: []const u8) !ListLanguagesResponse {
    const raw = c.greeting__v1__list_languages_response__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeListLanguagesResponseFailed;
    return .{ .raw = raw };
}

fn packMessage(allocator: std.mem.Allocator, base: *c.ProtobufCMessage, len: usize) ![]u8 {
    const buf = try allocator.alloc(u8, len);
    errdefer allocator.free(buf);
    const encoded_len = c.protobuf_c_message_pack(base, buf.ptr);
    if (encoded_len != len) return error.EncodeSizeMismatch;
    return buf;
}

fn anyValueString(value: ?*c.Holons__V1__AnyValue) ?[]const u8 {
    const raw = value orelse return null;
    if (raw.*.value_case != c.HOLONS__V1__ANY_VALUE__VALUE_STRING_VALUE) return null;
    return cstr(raw.*.unnamed_0.string_value);
}

fn logRecordAttributeString(raw: *c.Holons__V1__LogRecord, key: []const u8) ?[]const u8 {
    for (raw.*.attributes[0..raw.*.n_attributes]) |attr| {
        if (std.mem.eql(u8, cstr(attr.*.key), key)) return anyValueString(attr.*.value);
    }
    return null;
}

fn metricAttributeString(raw: *c.Holons__V1__Metric, key: []const u8) ?[]const u8 {
    const attrs = metricAttributes(raw) orelse return null;
    for (attrs.ptr[0..attrs.len]) |attr| {
        if (std.mem.eql(u8, cstr(attr.*.key), key)) return anyValueString(attr.*.value);
    }
    return null;
}

const AttributeSlice = struct {
    ptr: [*c][*c]c.Holons__V1__KeyValue,
    len: usize,
};

fn metricAttributes(raw: *c.Holons__V1__Metric) ?AttributeSlice {
    switch (raw.*.data_case) {
        c.HOLONS__V1__METRIC__DATA_GAUGE => {
            const gauge = raw.*.unnamed_0.gauge orelse return null;
            if (gauge.*.n_data_points == 0) return null;
            const point = gauge.*.data_points[0];
            return .{ .ptr = point.*.attributes, .len = point.*.n_attributes };
        },
        c.HOLONS__V1__METRIC__DATA_SUM => {
            const sum = raw.*.unnamed_0.sum orelse return null;
            if (sum.*.n_data_points == 0) return null;
            const point = sum.*.data_points[0];
            return .{ .ptr = point.*.attributes, .len = point.*.n_attributes };
        },
        c.HOLONS__V1__METRIC__DATA_HISTOGRAM => {
            const histogram = raw.*.unnamed_0.histogram orelse return null;
            if (histogram.*.n_data_points == 0) return null;
            const point = histogram.*.data_points[0];
            return .{ .ptr = point.*.attributes, .len = point.*.n_attributes };
        },
        else => return null,
    }
}

fn eventNameFromLegacy(value: i32) []const u8 {
    return switch (value) {
        1 => "instance.spawned",
        2 => "instance.ready",
        3 => "instance.exited",
        4 => "instance.crashed",
        5 => "session.started",
        6 => "session.ended",
        7 => "handler.panic",
        8 => "config.reloaded",
        else => "event.unspecified",
    };
}

fn cstr(ptr: [*c]const u8) []const u8 {
    if (ptr == null) return "";
    return std.mem.span(ptr);
}

test "protobuf-c runtime is available" {
    try std.testing.expect(version().number >= 1005002);
}
