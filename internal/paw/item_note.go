package paw

// Declare conformity to Item interface
var _ Item = (*Note)(nil)

type Note struct {
	Value     string `json:"value,omitempty"`
	*Metadata `json:"metadata,omitempty"`
}

func NewNote() *Note {
	return &Note{
		Metadata: &Metadata{
			Type: NoteItemType,
		},
	}
}
