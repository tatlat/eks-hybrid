package cli

import "github.com/integrii/flaggy"

type GlobalOptions struct {
	DevelopmentMode bool
}

func NewGlobalOptions() *GlobalOptions {
	opts := GlobalOptions{
		DevelopmentMode: false,
	}
	flaggy.Bool(&opts.DevelopmentMode, "d", "development", "Enable development mode for logging.")
	return &opts
}
