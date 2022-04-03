package cli

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"lucor.dev/paw/internal/age/bech32"
	"lucor.dev/paw/internal/paw"
)

// Sync sync a vault with a remote vault
type SyncCmd struct {
	vaultName string
	enable    bool
	syncKey   bool
	publicKey bool
}

// Name returns the one word command name
func (cmd *SyncCmd) Name() string {
	return "sync"
}

// Description returns the command description
func (cmd *SyncCmd) Description() string {
	return "Sync a vault"
}

// Usage displays the command usage
func (cmd *SyncCmd) Usage() {
	template := `Usage: paw-cli sync VAULT

{{ . }}

Options:
      --enable    	Enable sync mode
      --pub-key		Displays the sync key for the vault and exit
      --sync-key	Displays the public key for the vault and exit
  -h, --help    	Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *SyncCmd) Parse(args []string) error {
	flags, err := newCommonFlags()
	if err != nil {
		return err
	}

	flagSet.BoolVar(&cmd.enable, "enable", false, "")
	flagSet.BoolVar(&cmd.syncKey, "sync-key", false, "")
	flagSet.BoolVar(&cmd.publicKey, "pub-key", false, "")

	flagSet.Parse(args)
	if flags.Help || len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(0)
	}

	cmd.vaultName = flagSet.Arg(0)
	return nil
}

// Run runs the command
func (cmd *SyncCmd) Run(s paw.Storage) error {
	password, err := askPassword("Enter the vault password")
	if err != nil {
		return err
	}

	vault, err := s.LoadVault(cmd.vaultName, password)
	if err != nil {
		return err
	}

	if cmd.enable {
		remoteURL, err := ask("SSH Remote URL (i.e. git@example.com:user/paw-sync.git)")
		if err != nil {
			return err
		}

		branch, err := askWithDefault("branch", "main")
		if err != nil {
			return err
		}

		key, err := s.CreateSyncKey(vault)
		if err != nil {
			return err
		}

		err = paw.SyncInitGitRepo(s, vault, remoteURL, branch)
		if err != nil {
			return err
		}
		fmt.Printf("[✓] sync enabled for vault %q\n", vault.Name)
		return printPublicKey(key)
	}

	sk, err := s.LoadSyncKey(vault)
	if err != nil {
		return err
	}

	if cmd.syncKey {
		bpk, err := bech32.Encode("PAW-SYNC-KEY-", sk.Seed())
		if err != nil {
			return err
		}
		fmt.Printf("[✓] sync key: %s\n", bpk)
		return nil
	}

	if cmd.publicKey {
		return printPublicKey(sk)
	}

	signer, err := ssh.NewSignerFromKey(sk)
	if err != nil {
		return err
	}

	err = paw.SyncGitRepo(context.TODO(), s, vault, signer)
	if err != nil {
		return err
	}

	return nil
}

func printPublicKey(key ed25519.PrivateKey) error {
	sshPublicKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return err
	}

	fmt.Printf("[✓] SSH public key: %s", ssh.MarshalAuthorizedKey(sshPublicKey))
	return nil
}
