# Changelog - Paw

## 0.17.1 - 02 April 2022

- gui: fix incorrect value for the public key displayed into thr ssh key view 

## 0.17.0 - 29 March 2022

- all: add Ed25519 and RSA SSH keys support
- deps add:
    - github.com/mikesmitty/edkey v0.0.0-20170222072505-3356ea4e686a
- deps upgrade:
    - fyne.io/fyne v2.1.4
    - golang.org/x/crypto v0.0.0-20220321153916-2c7772ba3064
    - golang.org/x/image v0.0.0-20220321031419-a8550c1d254a
    - golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
    - golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
 
## 0.16.1 - 08 March 2022

- gui: fix item creation should show default content on cancel

## 0.16.0 - 28 February 2022 

- all: fix regression about setting item date
- cli: add the "-c, --clip" option to copy password to clipboard
- cli: update messages to printed correctly on stdout and stderr
- cli:list command will show an hint message if no vaults are found
- cli,deps: add golang.design/x/clipboard
- gui,deps: update fyne.io/fyne to v2.1.3 

## 0.15.0 - 26 January 2022 

- cli: add CLI application #3

## 0.14.0 - 09 January 2022

> This release updates the internal storage, so previous versions won't be compatible.
> Starting from this release the data is encoded in json in place of gob 
> and update to use a password protected age key (X25519) to decrypt and encrypt the vault data.
> This allow to decrypt the items using the age tool and have the content directly accessible once decrypted.

- paw: update to use a password protected age key (X25519) to decrypt and encrypt the vault data
- paw: data encoded in json in place of gob
- paw,ui: group vault ItemMetadata by ItemType
- paw,ui: export item UX improvement: items are now decoded concurrently and a progress bar is shown if needed
- ui: show item count into the item select list
- ui: fix renaming an item when a filter is specified could display the vault empty view

## 0.13.1 - 07 January 2022

- paw: item creation was not working correctly
- doc: update screenshot

## 0.13.0 - 07 January 2022

> This release updates the internal storage, so previous versions won't be compatible.

- paw: the website item has be renamed into login to make it more general purpose
- ui: support showing website favicons #8
- favicon: add package favicon that provides a favicon downloader #8
- ico: add package ico that implements a minimal ICO image decoder #8

## 0.12.0 - 03 January 2022

> This release updates the internal storage, so previous versions won't be compatible.

- paw,ui: import items from file #6
- paw,ui: export items to file #7
- haveibeenpwned,ui: improve password audit

## 0.11.0 - 28 December 2021

> This release updates the internal storage, so previous versions won't be compatible.

- paw: items are now stored into dedicated age files
- paw,ui: add passphrase support #4
- paw,ui: add pin password support

## 0.10.0 - 21 December 2021

### Added

- Password audit against data breaches #1 
- Add TOTP and HTOP support #5

## 0.9.0 - 11 December 2021

- First release
