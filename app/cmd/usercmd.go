package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth"
	"github.com/go-bumbu/userauth/service/user"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const configFlagUsage = "config file; default: ./config.yaml then /etc/aether/config.yaml"

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "manage native users",
	}
	cmd.AddCommand(
		userHashCmd(),
		userResetPasswordCmd(),
		userCreateCmd(),
		userListCmd(),
		userRoleCmd(),
	)
	return cmd
}

// cliAppCfg loads the app config the offline user commands run against, using
// the same search order as the server (explicit --config, else the default
// paths).
func cliAppCfg(configFile string) (AppCfg, error) {
	path, mandatory := resolveConfigFile(configFile, configSearchPaths)
	return getAppCfg(path, mandatory)
}

// openUsersStore opens the aether DB from cfg and builds the native identity
// service. The database must already exist — the server creates and migrates it
// on first start; the CLI edits an existing store rather than bootstrapping one,
// so a missing DB is a clear error instead of a stray empty file.
func openUsersStore(cfg AppCfg) (*user.Service, error) {
	dbPath := filepath.Join(cfg.DataDir, dbFile)
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("database not found at %s (wrong DataDir or server never started?)", dbPath)
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA busy_timeout=5000")
	users, err := newUserStore(db, nil)
	if err != nil {
		return nil, fmt.Errorf("user store: %w", err)
	}
	return users, nil
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
			cfg, err := cliAppCfg(configFile)
			if err != nil {
				return err
			}
			return resetUserPassword(cfg, login, pw)
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "", configFlagUsage)
	return cmd
}

// resetUserPassword replaces the password hash of an existing user. The user
// must already exist.
func resetUserPassword(cfg AppCfg, login, pw string) error {
	users, err := openUsersStore(cfg)
	if err != nil {
		return err
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

func userCreateCmd() *cobra.Command {
	var configFile string
	var admin bool
	cmd := &cobra.Command{
		Use:   "create <login> [password]",
		Short: "create a native user in the database",
		Long: "Create a native user directly in the database. Prompts for the password\n" +
			"when not given as an argument. Pass --admin to grant the admin role. The\n" +
			"server should be stopped, or at least not serving, while the DB is modified.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			login := args[0]
			pw, err := passwordArg(args[1:], "Password: ")
			if err != nil {
				return err
			}
			cfg, err := cliAppCfg(configFile)
			if err != nil {
				return err
			}
			return createNativeUser(cfg, login, pw, admin)
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "", configFlagUsage)
	cmd.Flags().BoolVar(&admin, "admin", false, "grant the admin role")
	return cmd
}

// createNativeUser adds a new native user, optionally as an admin. The login is
// refused if it collides with the usertoken PAT virtual-username namespace, the
// same rule the users CRUD enforces.
func createNativeUser(cfg AppCfg, login, pw string, admin bool) error {
	login = strings.TrimSpace(login)
	if login == "" {
		return errors.New("login cannot be empty")
	}
	if usersHandler.IsTokenShapedLogin(login) {
		return errors.New("login must not look like a token id (10 lowercase letters/digits)")
	}
	if pw == "" {
		return errors.New("password cannot be empty")
	}
	users, err := openUsersStore(cfg)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptDifficulty)
	if err != nil {
		return err
	}
	enabled := true
	role := usersHandler.RoleUser
	var groups []string
	if admin {
		role = usersHandler.RoleAdmin
		groups = []string{usersHandler.AdminGroup}
	}
	if _, err := users.CreateUser(user.Draft{
		LoginID:      login,
		PasswordHash: string(hash),
		Enabled:      &enabled,
		Groups:       groups,
	}); err != nil {
		if errors.Is(err, user.ErrLoginIDTaken) {
			return fmt.Errorf("user %q already exists", login)
		}
		return err
	}
	fmt.Printf("created %s %q\n", role, login)
	return nil
}

// userLine is one row of `aether user list`.
type userLine struct {
	Login   string
	Role    string
	Enabled bool
}

func userListCmd() *cobra.Command {
	var configFile string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list native users with their role and status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cliAppCfg(configFile)
			if err != nil {
				return err
			}
			lines, err := listNativeUsers(cfg)
			if err != nil {
				return err
			}
			if len(lines) == 0 {
				fmt.Println("no users")
				return nil
			}
			for _, l := range lines {
				status := "enabled"
				if !l.Enabled {
					status = "disabled"
				}
				fmt.Printf("%-24s  %-5s  %s\n", l.Login, l.Role, status)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "", configFlagUsage)
	return cmd
}

// listNativeUsers returns every user with its derived role, ordered by login
// (the store's order). Capped at 200 like the users CRUD — a self-hosted server
// does not have more.
func listNativeUsers(cfg AppCfg) ([]userLine, error) {
	users, err := openUsersStore(cfg)
	if err != nil {
		return nil, err
	}
	res, err := users.List(user.ListOpts{Limit: 200})
	if err != nil {
		return nil, err
	}
	lines := make([]userLine, 0, len(res.Users))
	for _, u := range res.Users {
		role, err := usersHandler.RoleOf(users, u.ID)
		if err != nil {
			return nil, err
		}
		lines = append(lines, userLine{Login: u.LoginID, Role: role, Enabled: u.Enabled})
	}
	return lines, nil
}

func userRoleCmd() *cobra.Command {
	var configFile string
	cmd := &cobra.Command{
		Use:   "role <login> <admin|user>",
		Short: "set a user's role (admin or user)",
		Long: "Grant or revoke the admin role of an existing native user. This is the\n" +
			"break-glass recovery path when no admin can log in. Promotion is always\n" +
			"allowed; demoting the last enabled admin is refused.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cliAppCfg(configFile)
			if err != nil {
				return err
			}
			return setUserRole(cfg, args[0], args[1])
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "", configFlagUsage)
	return cmd
}

// setUserRole grants or revokes the admin role of an existing user. Demoting the
// last enabled admin is refused, matching the users CRUD lockout guard, so the
// CLI cannot strand the server without an admin.
func setUserRole(cfg AppCfg, login, role string) error {
	if role != usersHandler.RoleAdmin && role != usersHandler.RoleUser {
		return fmt.Errorf("role must be %q or %q", usersHandler.RoleAdmin, usersHandler.RoleUser)
	}
	users, err := openUsersStore(cfg)
	if err != nil {
		return err
	}
	usr, err := users.GetUserByLogin(login)
	if err != nil {
		if errors.Is(err, userauth.ErrUserNotFound) {
			return fmt.Errorf("user %q not found", login)
		}
		return err
	}
	if role == usersHandler.RoleUser {
		last, err := usersHandler.IsLastEnabledAdmin(users, usr.ID)
		if err != nil {
			return err
		}
		if last {
			return errors.New("refusing to demote the last admin: promote another user first")
		}
	}
	if err := usersHandler.SetRole(users, usr.ID, role); err != nil {
		return err
	}
	fmt.Printf("user %q is now %s\n", login, role)
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
