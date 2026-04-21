#include "holons/holons.h"

static holons_field_doc_t holons_generated_input_fields[] = {
    {
        .name = "message",
        .type = "string",
        .number = 1,
        .description = "Message to echo.",
        .label = HOLONS_FIELD_LABEL_REQUIRED,
        .required = 1,
        .example = "\"hello\"",
    },
};

static holons_field_doc_t holons_generated_output_fields[] = {
    {
        .name = "message",
        .type = "string",
        .number = 1,
        .description = "Echoed message.",
        .label = HOLONS_FIELD_LABEL_OPTIONAL,
    },
};

static holons_method_doc_t holons_generated_methods[] = {
    {
        .name = "Ping",
        .description = "Replies with the payload.",
        .input_type = "static.v1.PingRequest",
        .output_type = "static.v1.PingResponse",
        .input_fields = holons_generated_input_fields,
        .input_field_count = 1,
        .output_fields = holons_generated_output_fields,
        .output_field_count = 1,
        .example_input = "{\"message\":\"hello\"}",
    },
};

static holons_service_doc_t holons_generated_services[] = {
    {
        .name = "static.v1.Echo",
        .description = "Static describe service.",
        .methods = holons_generated_methods,
        .method_count = 1,
    },
};

static holons_describe_response_t holons_generated_describe_response_value = {
    .manifest =
        {
            .identity =
                {
                    .uuid = "static-holon-0000",
                    .given_name = "Static",
                    .family_name = "Holon",
                    .motto = "Registered without runtime proto parsing.",
                    .composer = "describe-fixture",
                    .status = "draft",
                    .born = "2026-03-23",
                },
            .lang = "c",
            .kind = "native",
            .build =
                {
                    .runner = "cmake",
                    .main = "./cmd",
                },
            .artifacts =
                {
                    .binary = "describe-static-helper",
                },
        },
    .services = holons_generated_services,
    .service_count = 1,
};

const holons_describe_response_t *holons_generated_describe_response(void) {
  return &holons_generated_describe_response_value;
}
