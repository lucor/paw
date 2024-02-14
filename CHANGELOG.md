# Changelog - Paw

## 0.22.0 - 14 February 2024

- cli: disable clipboard on FreeBSD
- all: improve health service performance creating a lock file 
- all: update logo
- all: move main into project root
- all: detach console when running on Windows
- agent: update to use named pipe on Windows
- otp: ensure decoded key is padded
- otp: fix padding issue for the 2fa code
- ui: view could not refresh correctly using menu shortcuts 

- deps add:
    - gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce
- deps upgrade:
    - fyne.io/fyne v2.4.4

## 0.21.2 - 28 January 2024

- mobile: fix `undefined: clipboardWriteTimeout`

## 0.21.1 - 24 January 2024

- cli: disable CLI application on mobile OSes  
- ui: fix background color for the delete button in the edit view

## 0.21.0 - 21 January 2024

- all: merge CLI and GUI apps to provide only a binary
- deps upgrade:
    - fyne.io/fyne v2.4.3
	- golang.org/x/crypto v0.18.0
	- golang.org/x/image v0.15.0
	- golang.org/x/sync v0.6.0
	- golang.org/x/term v0.16.0
	- golang.org/x/text v0.14.0

## 0.20.1 - 15 November 2023

- ui: update the vault layout to focus the search box using shift+tab
- deps: update systray to fix a possible panic

## 0.20.0 - 09 November 2023

- agent: initial implementation of the server agent to handle SSH keys and CLI sessions
- agent: initial implementation of the client agent to manage CLI sessions
- cli,ui: add support for encrypted SSH keys with a passphrase for SSH item
- storage: add SocketAgentPath method to the Storage interface
- ui: update edit view to display a single action instead of the menu 
- deps upgrade:
    - filippo.io/age v1.1.1
    - fyne.io/fyne v2.4.1
    - fyne.io/systray v1.10.1-0.20231105182847-18ba13a8fe2b
    - golang.design/x/clipboard 0.7.0
    - golang.org/x/crypto v0.14.0
    - golang.org/x/sync v0.4.0
    - golang.org/x/image v0.13.0
    - golang.org/x/term v0.13.0
    - golang.org/x/text v0.13.0
- deps remove:
    - github.com/mikesmitty/edkey

## 0.19.1 - 01 October 2022

- ui: update preferences view to be scrollable
- ui: disable validation for the note entry

## 0.19.0 - 01 October 2022

- ui: quit from main menu does not quit the app
- ui: add preferences view #9
- ui: allow note entry to receive focus when tab is pressed
- ui: allow item list to receive focus when tab is pressed (via fyne upgrade)

- deps upgrade:
    - fyne.io/fyne v2.2.4-0.20221001083711-23d1052ad20e

## 0.18.0 - 21 September 2022

- ui, storage: initial support for mobile
- ui: systray initial implementation
- import: add ssh key type

- deps upgrade:
    - fyne.io/fyne v2.2.3
    - golang.design/x/clipboard v0.6.2
    - golang.org/x/image v0.0.0-20220601225756-64ec528b34cd
    - golang.org/x/text v0.3.7

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
