package mod

var importers = map[string]importer{}

type importer interface {
	String() string

	channelCount() int
	readPattern(Reader) (Pattern, error)
	reorder([]byte)
}
