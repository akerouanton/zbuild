package main

import (
	"log"

	"github.com/NiR-/webdf/pkg/deftypes/php"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/spf13/cobra"
)

var (
	webdfCmd *cobra.Command
)

func main() {
	webdfCmd = &cobra.Command{
		Use:               "webdf",
		DisableAutoGenTag: true,
		Short:             "webdf is a tool made to easily manage Docker-based environments and help developers working on web projects",
	}

	webdfCmd.AddCommand(newUpdateCmd())
	webdfCmd.AddCommand(newDebugLLBCmd())

	if err := webdfCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func buildTypeRegistry() *registry.TypeRegistry {
	reg := registry.NewTypeRegistry()
	php.RegisterDefType(reg)

	return reg
}
