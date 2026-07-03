package main

import (
	"github.com/aux-ai/aux-cli/cmd"
	"github.com/aux-ai/aux-cli/internal/logging"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.ErrorPersist("Application terminated due to unhandled panic")
	})

	cmd.Execute()
}
