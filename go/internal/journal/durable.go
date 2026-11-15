package journal

// Source identity is OS-stable across process restarts; path alone is not identity.
type Source struct {
	ID, Path, Identity string
	Size               int64
}
type Checkpoint struct {
	Source       Source
	Offset       int64
	Sequence     uint64
	Head, Anchor []byte
	Discard      bool
}
type Record struct {
	Event Event
	Phase Phase
}
type Durable interface {
	Resume(Source) (Checkpoint, error)
	CommitRecords([]Record, Checkpoint) error
}
