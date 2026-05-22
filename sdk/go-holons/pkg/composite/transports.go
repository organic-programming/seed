package composite

// TransportCoverageSequence is the canonical 10-phase walk that exercises
// every (from, to) transition between stdio, tcp and unix.
var TransportCoverageSequence = []string{
	"stdio", "stdio", "tcp", "unix", "tcp", "tcp",
	"stdio", "unix", "unix", "stdio",
}
