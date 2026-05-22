'use strict';

const LOCAL = 0;
const PROXY = 1;
const DELEGATED = 2;

const SIBLINGS = 0x01;
const CWD = 0x02;
const SOURCE = 0x04;
const BUILT = 0x08;
const INSTALLED = 0x10;
const CACHED = 0x20;
const ALL = 0x3F;

const NO_LIMIT = 0;
const NO_TIMEOUT = 0;

/**
 * @typedef {Object} IdentityInfo
 * @property {string} given_name
 * @property {string} family_name
 * @property {string} [motto]
 * @property {string[]} [aliases]
 */

/**
 * @typedef {Object} HolonInfo
 * @property {string} slug
 * @property {string} uuid
 * @property {IdentityInfo} identity
 * @property {string} lang
 * @property {string} runner
 * @property {string} status
 * @property {string} kind
 * @property {string} transport
 * @property {string} entrypoint
 * @property {string[]} architectures
 * @property {boolean} has_dist
 * @property {boolean} has_source
 */

/** @typedef {{ url: string, info: HolonInfo|null, error: string|null }} HolonRef */
/** @typedef {{ found: HolonRef[], error: string|null }} DiscoverResult */
/** @typedef {{ ref: HolonRef|null, error: string|null }} ResolveResult */
/** @typedef {{ channel: object|null, uid: string, origin: HolonRef|null, error: string|null }} ConnectResult */

module.exports = {
    LOCAL,
    PROXY,
    DELEGATED,
    SIBLINGS,
    CWD,
    SOURCE,
    BUILT,
    INSTALLED,
    CACHED,
    ALL,
    NO_LIMIT,
    NO_TIMEOUT,
};
