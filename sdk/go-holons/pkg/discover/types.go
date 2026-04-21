package discover

// Scope flags.
const (
	LOCAL     = 0
	PROXY     = 1
	DELEGATED = 2
)

// Layer flags.
const (
	SIBLINGS  = 0x01
	CWD       = 0x02
	SOURCE    = 0x04
	BUILT     = 0x08
	INSTALLED = 0x10
	CACHED    = 0x20
	ALL       = 0x3F
)

// Clarity constants.
const (
	NO_LIMIT   = 0
	NO_TIMEOUT = 0
)

type HolonInfo struct {
	Slug          string       `json:"slug"`
	UUID          string       `json:"uuid"`
	Identity      IdentityInfo `json:"identity"`
	Lang          string       `json:"lang"`
	Runner        string       `json:"runner"`
	Status        string       `json:"status"`
	Kind          string       `json:"kind"`
	Transport     string       `json:"transport"`
	Entrypoint    string       `json:"entrypoint"`
	Architectures []string     `json:"architectures"`
	HasDist       bool         `json:"has_dist"`
	HasSource     bool         `json:"has_source"`

	// Internal fields used by connect/launch logic.
	BuildMain  string `json:"-"`
	SourceKind string `json:"-"`
}

type IdentityInfo struct {
	GivenName  string   `json:"given_name"`
	FamilyName string   `json:"family_name"`
	Motto      string   `json:"motto,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
}

type HolonRef struct {
	URL   string     `json:"url"`
	Info  *HolonInfo `json:"info"`
	Error string     `json:"error,omitempty"`
}

type DiscoverResult struct {
	Found []HolonRef `json:"found"`
	Error string     `json:"error,omitempty"`
}

type ResolveResult struct {
	Ref   *HolonRef `json:"ref"`
	Error string    `json:"error,omitempty"`
}
