package cli

import "flag"

var flagSet = flag.NewFlagSet("paw", flag.ExitOnError)

// CommonFlags holds the flags shared between all commands
type CommonFlags struct {
	// Help displays the command help and exit
	Help bool
}

// newCommonFlags defines all the flags for the shared options
func newCommonFlags() (*CommonFlags, error) {
	flags := &CommonFlags{}
	flagSet.BoolVar(&flags.Help, "help", false, "")
	flagSet.BoolVar(&flags.Help, "h", false, "")
	return flags, nil
}
