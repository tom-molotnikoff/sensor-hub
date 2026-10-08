package cmd

import (
	"bufio"
	"errors"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var localAdminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage administrators on this install",
}

var localAdminCreateCmd = &cobra.Command{
	Use:   "create <username>",
	Short: "Create the first admin user",
	Long: "Creates an admin user directly in the database; the server does not need to be running.\n" +
		"Refuses when any user already holds the admin role.\n\n" +
		"The password is read from stdin when stdin is not a terminal (one line), otherwise it is prompted for twice.",
	Args: cobra.ExactArgs(1),
	RunE: runLocalAdminCreate,
}

var adminCreateEmail string
var adminCreateMustChangePassword bool

func init() {
	localAdminCreateCmd.Flags().StringVar(&adminCreateEmail, "email", "", "Email address for the admin")
	localAdminCreateCmd.Flags().BoolVar(&adminCreateMustChangePassword, "must-change-password", false, "Require a password change at first login")
	localAdminCmd.AddCommand(localAdminCreateCmd)
	localCmd.AddCommand(localAdminCmd)
}

func runLocalAdminCreate(cmd *cobra.Command, args []string) error {
	logger := useWarningLogger(cmd)
	if err := appProps.InitialiseConfig(localConfigDir); err != nil {
		return fmt.Errorf("failed to initialise application configuration: %w", err)
	}

	password, err := readNewPassword(cmd)
	if err != nil {
		return err
	}
	hash, err := service.HashFirstAdminPassword(password)
	if err != nil {
		return err
	}

	db, err := database.Open(appProps.AppConfig(), logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	username := args[0]
	users := database.NewUserRepository(db, logger)
	admin := gen.User{Username: username, Email: adminCreateEmail, MustChangePassword: adminCreateMustChangePassword}
	if _, err := users.CreateFirstAdmin(cmd.Context(), admin, hash); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), username)
	return nil
}

func readNewPassword(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return promptNewPassword(cmd, int(f.Fd()))
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read password from stdin: %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func promptNewPassword(cmd *cobra.Command, fd int) (string, error) {
	prompt := func(label string) (string, error) {
		fmt.Fprint(cmd.ErrOrStderr(), label)
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return string(b), nil
	}
	first, err := prompt("Password: ")
	if err != nil {
		return "", err
	}
	second, err := prompt("Confirm password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("passwords do not match")
	}
	return first, nil
}
