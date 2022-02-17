package paw

// Declare conformity to Item interface
var _ Item = (*Login)(nil)

type Login struct {
	*Password `json:"password,omitempty"`
	*TOTP     `json:"totp,omitempty"`
	*Note     `json:"note,omitempty"`
	*Metadata `json:"metadata,omitempty"`

	Username string `json:"username,omitempty"`
	URL      string `json:"url,omitempty"`
}

func NewLogin() *Login {
	return &Login{
		Metadata: &Metadata{
			Type: LoginItemType,
		},
		Note:     &Note{},
		Password: &Password{},
		TOTP:     &TOTP{},
	}
}
