package paw

import "os"

const (
	ENV_DEBUG   = "PAW_DEBUG"   // The env var name can be used to enable debug
	ENV_HOME    = "PAW_HOME"    // The env var name can be used to override the Paw HOME directory
	ENV_SESSION = "PAW_SESSION" // The env var name can be used to specify a Paw session ID
)

// IsDebug returns true if the debug mode is enabled
func IsDebug() bool {
	return os.Getenv(ENV_DEBUG) == "true"
}
