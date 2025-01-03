package valkey

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Address string

func (a Address) parse() (string, int, error) {
	addr := string(a)
	lastColonIndex := strings.LastIndex(addr, ":")
	if lastColonIndex == -1 {
		return "", 0, fmt.Errorf("Invalid IP:Port format")
	}

	ip := strings.TrimSuffix(strings.TrimPrefix(addr[:lastColonIndex], "["), "]")
	port, err := strconv.Atoi(addr[lastColonIndex+1:])
	if err != nil {
		return "", 0, fmt.Errorf("Invalid port number: %v", err)
	}
	return ip, port, nil
}

func (a Address) Host() string {
	host, _, _ := a.parse()
	return host
}

func (a Address) Port() int {
	_, port, _ := a.parse()
	return port
}

func (a Address) String() string {
	host, port, _ := a.parse()
	return net.JoinHostPort(host, strconv.Itoa(port))
}
