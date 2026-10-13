package command

import (
	"errors"
	"fmt"
	"strings"

	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/persistence/purge"

	"github.com/spf13/cobra"
)

func NewPurgeCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:           "purge",
		Short:         "Empty every table except " + strings.Join(purge.KeptTables, " and "),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db := config.ConnectDatabaseURL(config.LoadCLI().DatabaseURL)
			if !force {
				tables, err := purge.Tables(db)
				if err != nil {
					return err
				}
				fmt.Printf("Would empty %d tables: %s\n", len(tables), strings.Join(tables, ", "))
				return errors.New("refusing to purge without --force")
			}
			tables, err := purge.Run(db)
			if err != nil {
				return err
			}
			fmt.Printf("Emptied %d tables: %s\n", len(tables), strings.Join(tables, ", "))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "actually truncate the tables")
	return cmd
}
