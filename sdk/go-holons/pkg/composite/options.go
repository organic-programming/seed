package composite

type dialOptions struct {
	transitiveObservability *bool
}

// DialOption configures SDK-created member connections.
type DialOption func(*dialOptions)

// WithTransitiveObservability toggles transitive observability for connections
// created by SpawnMember and Dial. Default for SpawnMember is true; default
// for plain Dial is false.
func WithTransitiveObservability(enabled bool) DialOption {
	return func(o *dialOptions) {
		o.transitiveObservability = &enabled
	}
}

func applyDialOptions(opts []DialOption) dialOptions {
	var out dialOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}
