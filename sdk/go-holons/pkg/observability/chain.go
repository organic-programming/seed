package observability

// AppendDirectChild returns a new chain equal to src followed by the
// direct child's slug. LogRecord.chain is a slug path from emitter to root.
func AppendDirectChild(src []string, childSlug string) []string {
	out := make([]string, 0, len(src)+1)
	out = append(out, src...)
	if childSlug != "" {
		out = append(out, childSlug)
	}
	return out
}

// EnrichForMultilog returns a copy of wire with the stream source appended.
func EnrichForMultilog(wire []string, streamSourceSlug string) []string {
	return AppendDirectChild(wire, streamSourceSlug)
}

// CloneChain returns an independent copy of the chain.
func CloneChain(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}
