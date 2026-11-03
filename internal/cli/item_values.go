package cli

import "go.lucor.dev/cxf"

// passwordValue returns the password value when the credential has one.
func passwordValue(credential cxf.BasicAuth) string {
	if credential.Password == nil {
		return ""
	}
	return credential.Password.Value
}

// usernameValue returns the username value when the credential has one.
func usernameValue(credential cxf.BasicAuth) string {
	if credential.Username == nil {
		return ""
	}
	return credential.Username.Value
}
