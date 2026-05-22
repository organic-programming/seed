'use strict';

const STATIC_DESCRIBE_ENV = 'HOLONS_STATIC_DESCRIBE_RESPONSE';
const UNSET = Symbol('unset');

function sampleStaticDescribeResponse() {
    return {
        manifest: {
            identity: {
                schema: 'holon/v1',
                uuid: 'js-holons-test-0000',
                given_name: 'JS',
                family_name: 'Holons',
                motto: 'Static describe for test fixtures.',
                composer: 'js-holons-test',
                status: 'draft',
                born: '2026-03-23',
            },
            lang: 'js',
        },
        services: [{
            name: 'echo.v1.Echo',
            description: 'Echo service for JS SDK tests.',
            methods: [{
                name: 'Ping',
                input_type: 'echo.v1.PingRequest',
                output_type: 'echo.v1.PingResponse',
            }],
        }],
    };
}

function useStaticDescribeResponse(t, describeModule, response = sampleStaticDescribeResponse()) {
    describeModule.useStaticResponse(response);
    t.after(() => {
        describeModule.useStaticResponse(null);
    });
    return response;
}

function useStaticDescribeEnv(t, response = sampleStaticDescribeResponse()) {
    const previous = Object.prototype.hasOwnProperty.call(process.env, STATIC_DESCRIBE_ENV)
        ? process.env[STATIC_DESCRIBE_ENV]
        : UNSET;

    process.env[STATIC_DESCRIBE_ENV] = JSON.stringify(response);
    t.after(() => {
        if (previous === UNSET) {
            delete process.env[STATIC_DESCRIBE_ENV];
            return;
        }
        process.env[STATIC_DESCRIBE_ENV] = previous;
    });

    return response;
}

module.exports = {
    STATIC_DESCRIBE_ENV,
    sampleStaticDescribeResponse,
    useStaticDescribeResponse,
    useStaticDescribeEnv,
};
