package llama

import "net"

// FreePort returns an available TCP port on the loopback interface.
// The listener is closed before returning; callers should still handle a
// possible bind race when starting their server.
func FreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
