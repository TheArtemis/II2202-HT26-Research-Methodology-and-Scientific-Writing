package raftnode

// T3SeedLogs matches the research-plan constrained-election initial log terms.
var T3SeedLogs = map[string][]uint64{
	"A": {1, 1},
	"B": {1, 1, 2},
	"C": {1, 1, 2, 2},
	"D": {1, 1, 2},
	"E": {1, 1},
}

// FormatSeedLog renders terms as a -seed-log flag value.
func FormatSeedLog(terms []uint64) string {
	if len(terms) == 0 {
		return ""
	}
	b := make([]byte, 0, len(terms)*2)
	for i, t := range terms {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, itoaUint(t)...)
	}
	return string(b)
}

func itoaUint(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
