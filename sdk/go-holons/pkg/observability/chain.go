package observability

// Hop mirrors the proto ChainHop. A LogEntry/EventInfo/MetricSample
// carries an ordered list of hops from the originator up to but not
// including the emitter of the stream the reader is consuming. See
// OBSERVABILITY.md §Organism Relay.
type Hop struct {
	Slug        string
	InstanceUID string
}

// AppendDirectChild returns a new chain equal to src followed by the
// identity of the direct child the SDK just read from. Used by the
// relay path: a holon that receives an entry on child.Logs adds the
// child's identity before re-emitting on its own stream.
func AppendDirectChild(src []Hop, childSlug, childUID string) []Hop {
	out := make([]Hop, len(src)+1)
	copy(out, src)
	out[len(src)] = Hop{Slug: childSlug, InstanceUID: childUID}
	return out
}

// EnrichForMultilog returns a copy of wire with the stream source
// appended. Used by the organism root when writing the multilog: the
// wire chain omits the stream source (known implicitly from which
// child.Logs was being read); the multilog entry stands alone, so it
// captures the full relay path. See OBSERVABILITY.md
// §Multilog chain enrichment.
func EnrichForMultilog(wire []Hop, streamSourceSlug, streamSourceUID string) []Hop {
	return AppendDirectChild(wire, streamSourceSlug, streamSourceUID)
}

// CloneHops returns an independent copy of the slice.
func CloneHops(src []Hop) []Hop {
	if len(src) == 0 {
		return nil
	}
	out := make([]Hop, len(src))
	copy(out, src)
	return out
}
