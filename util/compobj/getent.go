package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// errGetentNotFound is the error of a lookup of a name the database does not
// hold.
var errGetentNotFound = errors.New("not found")

// getentCommand returns the command looking a name up in a database of the
// name service switch, replaced by the tests.
var getentCommand = func(database, name string) *exec.Cmd {
	return exec.Command("getent", database, "--", name)
}

// getentID returns the id of a name of the passwd or group database, as the
// name service switch resolves it.
//
// The name is a value of a rule: it is an argument of getent, behind a "--",
// and not a word of a shell command line.
func getentID(database, name string) (int, error) {
	output, err := getentCommand(database, name).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return -1, fmt.Errorf("%s %s: %w", database, name, errGetentNotFound)
		}
		return -1, fmt.Errorf("getent %s %s: %w", database, name, err)
	}
	line, _, _ := strings.Cut(string(output), "\n")
	fields := strings.Split(line, ":")
	if len(fields) < 3 {
		return -1, fmt.Errorf("getent %s %s: unexpected output %q", database, name, line)
	}
	id, err := strconv.Atoi(fields[2])
	if err != nil {
		return -1, fmt.Errorf("getent %s %s: %w", database, name, err)
	}
	return id, nil
}

// checkAccountName refuses a user or group name of a rule that the account
// tools would read as an option rather than as a name: one starting with a
// '-' past the leading '-' marking a name to delete, when the rule allows
// that marker. A name holding a ':' or a line break is refused too: it would
// end the field, or the record, of the account files and of the lines fed to
// chpasswd.
func checkAccountName(name string, deletable bool) error {
	s := name
	if deletable {
		s = strings.TrimPrefix(s, "-")
	}
	if s == "" || strings.HasPrefix(s, "-") || strings.ContainsAny(s, ":\r\n") {
		return fmt.Errorf("invalid user or group name %q", name)
	}
	return nil
}

// checkPasswordHash refuses a password hash holding a ':' or a line break,
// which no hash holds: written to chpasswd, it would end the record, and the
// next line would set the password of another account.
func checkPasswordHash(s string) error {
	if strings.ContainsAny(s, ":\r\n") {
		return fmt.Errorf("invalid password hash: a hash holds no ':' nor line break")
	}
	return nil
}
