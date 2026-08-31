package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "manage native users",
	}
	cmd.AddCommand(
		userHashCmd(),
		userResetPasswordCmd(),
	)
	return cmd
}

func userHashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hash [password]",
		Short: "print the bcrypt hash of a password",
		Long: "Print the bcrypt hash of a password, usable as Auth.AdminBootstrap.Pw in\n" +
			"the config or via AETHER_AUTH_ADMINBOOTSTRAP_PW. Prompts when no argument is\n" +
			"given (preferred: the password stays out of the shell history).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pw, err := passwordArg(args, "Password: ")
			if err != nil {
				return err
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptDifficulty)
			if err != nil {
				return err
			}
			fmt.Println(string(hash))
			return nil
		},
	}
	return cmd
}

func userResetPasswordCmd() *cobra.Command {
	var configFile string
	cmd := &cobra.Command{
		Use:   "reset-password <login> [password]",
		Short: "reset a user's password in the database",
		Long: "Reset the password of an existing native user. Prompts for the new password\n" +
			"when not given as an argument. The server should be stopped, or at least not\n" +
			"serving the affected user, while the database is modified.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			login := args[0]
			pw, err := passwordArg(args[1:], "New password: ")
			if err != nil {
				return err
			}
			path, mandatory := resolveConfigFile(configFile, configSearchPaths)
			cfg, err := getAppCfg(path, mandatory)
			if err != nil {
				return err
			}
			return resetUserPassword(cfg, login, pw)
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "",
		"config file; default: ./config.yaml then /etc/aether/config.yaml")
	return cmd
}

// resetUserPassword opens the aether DB from cfg and replaces the password
// hash of an existing user. The user must already exist.
func resetUserPassword(cfg AppCfg, login, pw string) error {
	dbPath := filepath.Join(cfg.DataDir, dbFile)
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("database not found at %s (wrong DataDir or server never started?)", dbPath)
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		return err
	}
	db.Exec("PRAGMA busy_timeout=5000")

	users, err := newUserStore(db, nil)
	if err != nil {
		return fmt.Errorf("user store: %w", err)
	}
	// The CLI works with the login name; mutations key on the stable UUID.
	usr, err := users.GetUserByLogin(login)
	if err != nil {
		if errors.Is(err, userauth.ErrUserNotFound) {
			return fmt.Errorf("user %q not found", login)
		}
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptDifficulty)
	if err != nil {
		return err
	}
	if err := users.SetPasswordHash(usr.ID, string(hash)); err != nil {
		return err
	}
	fmt.Printf("password updated for user %q\n", login)
	return nil
}

// passwordArg returns the password from args when given, otherwise prompts on
// the terminal (hidden input, asked twice) or reads one line from stdin when
// input is piped.
func passwordArg(args []string, prompt string) (string, error) {
	if len(args) == 1 {
		if args[0] == "" {
			return "", errors.New("password cannot be empty")
		}
		return args[0], nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		pw := strings.TrimRight(line, "\r\n")
		if pw == "" {
			return "", errors.New("password cannot be empty")
		}
		return pw, nil
	}
	fmt.Print(prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if len(pw) == 0 {
		return "", errors.New("password cannot be empty")
	}
	fmt.Print("Repeat password: ")
	pw2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if string(pw) != string(pw2) {
		return "", errors.New("passwords do not match")
	}
	return string(pw), nil
}
