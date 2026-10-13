package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"go-api/cmd/cli/di"
	appvariable "go-api/internal/application/command/variable"
	appworkflow "go-api/internal/application/command/workflow"
	"go-api/internal/infrastructure/crypto"

	"github.com/spf13/cobra"
)

func NewWorkflowImportCommand() *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use:           "workflow:import <file.yaml>",
		Short:         "Create a workflow from a YAML definition",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			def, err := parseWorkflowFile(data)
			if err != nil {
				return err
			}

			container, err := di.NewContainer()
			if err != nil {
				return err
			}
			ctx := context.Background()
			userID, err := container.ResolveUser(ctx, user)
			if err != nil {
				return err
			}

			w, err := container.CreateWorkflow.Handle(ctx, appworkflow.CreateWorkflowCommand{UserID: userID, Definition: def})
			if err != nil {
				return err
			}
			fmt.Printf("Workflow %q created: %s (%d calls, every %s)\n", w.Name, w.ID, len(w.Calls), w.Interval)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "owner, as a user UUID or Clerk ID")
	_ = cmd.MarkFlagRequired("user")
	return cmd
}

func NewVariableSetCommand() *cobra.Command {
	var user string
	var secret bool
	cmd := &cobra.Command{
		Use:           "variable:set <name> <value>",
		Short:         "Create or replace a global variable (use - as value to read stdin)",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			value := args[1]
			if value == "-" {
				raw, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				value = strings.TrimRight(string(raw), "\r\n")
			}

			container, err := di.NewContainer()
			if err != nil {
				return err
			}
			ctx := context.Background()
			userID, err := container.ResolveUser(ctx, user)
			if err != nil {
				return err
			}
			g, err := container.SetGlobalVariable.Handle(ctx, appvariable.SetGlobalVariableCommand{
				UserID: userID, Name: args[0], Value: value, Secret: secret,
			})
			if err != nil {
				return err
			}
			kind := "variable"
			if g.Secret {
				kind = "secret variable"
			}
			fmt.Printf("Global %s {{%s}} saved\n", kind, g.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "owner, as a user UUID or Clerk ID")
	cmd.Flags().BoolVar(&secret, "secret", false, "encrypt the value and mask it everywhere")
	_ = cmd.MarkFlagRequired("user")
	return cmd
}

func NewKeysGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "keys:generate",
		Short: "Print new SECRETS_ENCRYPTION_KEY and JOB_SEAL key pair",
		RunE: func(cmd *cobra.Command, args []string) error {
			secretKey, err := crypto.GenerateSecretKey()
			if err != nil {
				return err
			}
			public, private, err := crypto.GenerateSealKeys()
			if err != nil {
				return err
			}
			fmt.Printf("SECRETS_ENCRYPTION_KEY=%s\nJOB_SEAL_PUBLIC_KEY=%s\nJOB_SEAL_PRIVATE_KEY=%s\n", secretKey, public, private)
			return nil
		},
	}
}
