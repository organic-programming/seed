/**
 * Shared discovery constants and JSDoc types for js-web-holons.
 */

export const LOCAL = 0;
export const PROXY = 1;
export const DELEGATED = 2;

export const SIBLINGS = 0x01;
export const CWD = 0x02;
export const SOURCE = 0x04;
export const BUILT = 0x08;
export const INSTALLED = 0x10;
export const CACHED = 0x20;
export const ALL = 0x3F;

export const NO_LIMIT = 0;
export const NO_TIMEOUT = 0;

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

/**
 * @typedef {Object} HolonRef
 * @property {string} url
 * @property {HolonInfo|null} info
 * @property {string|null} error
 */

/**
 * @typedef {Object} DiscoverResult
 * @property {HolonRef[]} found
 * @property {string|null} error
 */

/**
 * @typedef {Object} ResolveResult
 * @property {HolonRef|null} ref
 * @property {string|null} error
 */

/**
 * @typedef {Object} ConnectResult
 * @property {object|null} channel
 * @property {string} uid
 * @property {HolonRef|null} origin
 * @property {string|null} error
 */
