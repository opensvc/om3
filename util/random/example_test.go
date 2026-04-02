package random_test

import (
	"fmt"
	"github.com/opensvc/om3/v3/util/random"
)

func ExamplePassword() {
	// Generate a 16-character password using default charset
	pwd1 := random.Password(16, nil)
	fmt.Printf("Password length: %d\n", len(pwd1))

	// Generate a 12-character alphanumeric password
	alphanumeric := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	pwd2 := random.Password(12, alphanumeric)
	fmt.Printf("Alphanumeric password length: %d\n", len(pwd2))

	// Generate an 8-character numeric PIN
	numeric := []rune("0123456789")
	pin := random.Password(8, numeric)
	fmt.Printf("Numeric PIN length: %d\n", len(pin))

	// Output:
	// Password length: 16
	// Alphanumeric password length: 12
	// Numeric PIN length: 8
}