package util

import (
	"fmt"
	"net"
	"strconv"
)

// CheckHostPort is a utility function that returns an Error if address
// is not in the form host:port, or if the port is not from 1 to 65535.
// This allows an empty host (like :50051), which listening addresses
// use to listen on all network interfaces.
//
// NOTE: This only checks the format of the address. This does not check
// if the port is free or if the host can be reached.
func CheckHostPort(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", address, err)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("invalid address %q: port must be a number from 1 to 65535", address)
	}

	return nil
}
