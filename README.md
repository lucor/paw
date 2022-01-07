<div align="center">
    <img alt="Paw" src="logo/paw.png" height="128" />
</div>

# Paw

Paw is a cross platform application to manage your passwords and identities securely.

It is written in Go and uses [Fyne](https://github.com/fyne-io/fyne) as UI toolkit and [age](https://github.com/FiloSottile/age) as encryption library.

## Warning

**This software is work in progress, and didn't went through a full security audit.**

**Do not expect it to be bug free and do not rely on it for any type of security.**

## Screenshot

<div align="center">
    <img alt="Paw screenshot" src="screenshot.png" />
</div>

## Main features

* Cross platform application (linux, macOS, Windows, BSD ...) with a single codebase
* Minimal direct dependencies
* Open source: code can be audited
* Only one secret key to remember used to store securely your passwords
* Audit passwords against data breaches
* TOTP support
* Password import/export

### Later goals

* Automatically detect and use password rules for known web sites that require ones
* Automatic backup / syncronization
* CLI application
* Mobile / Web applications
* Stateless password derivation support
* Unicode password support

## Installation

### Stable version

```
go install lucor.dev/paw/cmd/paw@latest
```

### Development version

To try the development version or help with testing:

```
go install lucor.dev/paw/cmd/paw@develop
```

## How it works - cryptography details

### Vault initialization

One or more vaults can be initialized to store passwords and identities.

When the vault is initialized user will be prompt for a vault name and password that are used for:
- generate an [age](https://github.com/FiloSottile/age) Scrypt Identity and Recipient used to decrypt/encrypt the vault data;
- derive a symmetric secret key with [Scrypt](https://pkg.go.dev/golang.org/x/crypto/scrypt) used as seed for the random password generation;

### Random password

Random password are derived reading byte-by-byte the block of randomness from a [HKDF](https://pkg.go.dev/golang.org/x/crypto/hkdf) cryptographic key derivation function that uses the seed above as secret. Printable characters that match the desired password rule (uppercase, lowercase, symbols and digits) are then included in the generated password.

### Custom password

Where a generated password is not applicable a custom password can be specified. 

### Vault structure

Vault internally is organized hierarchically like:
```
- vault
    ├── login
    |    └── www.example.com
    |    └── my.site.com
    ├── password
    |    └── mypassword
    └── note
         └── mysecretnote
```

where login, password and note are the Paw items, see the dedicated section for details.

### Items

Items are special templates aim to help the identity management.

Currently the following items are available:

- login
- note
- password

## Threat model

The threat model of Paw assumes there are no attackers on your local machine.

## Contribute

- Fork and clone the repository
- Make and test your changes
- Open a pull request against the `develop` branch

## Contributors

See [contributors](https://github.com/lucor/paw/graphs/contributors) page

## Credits

 - [age](https://github.com/FiloSottile/age) for the encryption library
 - [Fyne](https://github.com/fyne-io/fyne) for the UI toolkit

Also thanks to these Open Source password managers that inspired Paw:

- [gopass](https://github.com/gopasspw/gopass)
- [lesspass](https://github.com/lesspass/lesspass)
- [pass](https://www.passwordstore.org/)
- [passgo](https://github.com/ejcx/passgo)
