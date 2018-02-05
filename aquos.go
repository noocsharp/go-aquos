// Package aquos provides a client to connect to AQUOS.
package aquos

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var DefaultLoginTimeout = 200 * time.Millisecond

// A Client represents a client to connect to AQUOS.
type Client struct {
	Username     string
	Password     string
	LoginTimeout time.Duration

	conn net.Conn
	w    *bufio.Writer
	res  chan response

	name              string
	modelName         string
	softwareVersion   string
	ipProtocolVersion string
}

type response struct {
	text string
	err  error
}

// Connect connects to the address on the named network using the provided context.
func (c *Client) Connect(ctx context.Context, address string) error {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	c.conn = conn
	c.w = bufio.NewWriter(conn)

	c.res = make(chan response)
	go c.readLoop()

	err = c.login()
	if err != nil {
		return err
	}

	err = c.getInfo()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) readLoop() {
	defer func() {
		close(c.res)
	}()

	s := bufio.NewScanner(c.conn)
	s.Split(scanLines)

	for {
		if s.Scan() {
			c.res <- response{
				text: s.Text(),
			}
		} else {
			c.res <- response{
				err: s.Err(),
			}
			return
		}
	}
}

func (c *Client) login() error {
	var err error

	timeout := c.LoginTimeout
	if timeout <= 0 {
		timeout = DefaultLoginTimeout
	}

	// wait login
	select {
	case <-time.After(timeout):
		// time out (login not required)
		return nil
	case r := <-c.res:
		if r.err != nil {
			return r.err
		}
		if !strings.Contains(r.text, "Login") {
			return errors.New("failed to login (invalid response)")
		}
		if c.Username == "" {
			return errors.New("username is not specified")
		}

		// send username
		err = c.send(c.Username)
		if err != nil {
			return err
		}
	}

	// wait password
	select {
	case <-time.After(timeout):
		return errors.New("failed to login (AQUOS does not respond)")
	case r := <-c.res:
		if r.err != nil {
			return r.err
		}
		if !strings.Contains(r.text, "Password") {
			return errors.New("failed to login (invalid response)")
		}
		if c.Password == "" {
			return errors.New("Password is not specified")
		}

		// send password
		err = c.send(c.Password)
		if err != nil {
			return err
		}
	}

	select {
	case <-time.After(timeout):
		// login success
	case r := <-c.res:
		if r.err != nil {
			return r.err
		}
		// login failed
		return fmt.Errorf("failed to login (%s)", r.text)
	}

	return nil
}

func (c *Client) getInfo() error {
	var err error

	c.name, err = c.sendCommand("TVNM", "1")
	if err != nil {
		return err
	}
	c.modelName, err = c.sendCommand("MNRD", "1")
	if err != nil {
		return err
	}
	c.softwareVersion, err = c.sendCommand("SWVN", "1")
	if err != nil {
		return err
	}
	c.ipProtocolVersion, err = c.sendCommand("IPPV", "1")
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) sendCommand(cmd, arg string) (res string, err error) {
	err = c.send(fmt.Sprintf("%s%-4s", cmd, arg))
	if err != nil {
		return
	}
	res, err = c.readLine()
	if err != nil {
		return
	}
	if res == "ERR" {
		err = errors.New("aquos returns a error")
		return
	}

	return
}

func (c *Client) send(str string) (err error) {
	_, err = c.w.WriteString(str)
	if err != nil {
		return
	}

	// ret code
	c.w.WriteByte('\r')
	if err != nil {
		return
	}

	err = c.w.Flush()
	if err != nil {
		return
	}

	return
}

func (c *Client) readLine() (string, error) {
	r, ok := <-c.res
	if !ok {
		return "", errors.New("connection already closed")
	}
	if r.err != nil {
		return "", r.err
	}

	return r.text, nil
}

func isIgnore(b byte) bool {
	return b == '\r' || b == '\n' || b == ':'
}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	start := 0
	for ; start < len(data); start++ {
		if !isIgnore(data[start]) {
			break
		}
	}

	for i := start; i < len(data); i++ {
		if isIgnore(data[i]) {
			return i + 1, data[start:i], nil
		}
	}

	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}

	return start, nil, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) ModelName() string {
	return c.modelName
}

func (c *Client) SoftwareVersion() string {
	return c.softwareVersion
}

func (c *Client) IPProtocolVersion() string {
	return c.ipProtocolVersion
}

func (c *Client) Power(on bool) error {
	arg := "0"
	if on {
		arg = "1"
	}

	_, err := c.sendCommand("POWR", arg)
	return err
}

func (c *Client) ToggleInput() error {
	_, err := c.sendCommand("ITGD", "-")
	return err
}

func (c *Client) ChangeInputTV() error {
	_, err := c.sendCommand("ITVD", "-")
	return err
}

func (c *Client) ChangeInput(source int) error {
	arg := strconv.Itoa(source)
	_, err := c.sendCommand("IAVD", arg)
	return err
}

func (c *Client) ChannelUp() error {
	_, err := c.sendCommand("CHUP", "-")
	return err
}

func (c *Client) ChannelDown() error {
	_, err := c.sendCommand("CHDW", "-")
	return err
}

func (c *Client) SetVolume(volume int) error {
	arg := strconv.Itoa(volume)
	_, err := c.sendCommand("VOLM", arg)
	return err
}

func (c *Client) Volume() (int, error) {
	res, err := c.sendCommand("VOLM", "?")
	if err != nil {
		return 0, err
	}

	volume, err := strconv.Atoi(res)
	if err != nil {
		return 0, err
	}

	return volume, nil
}
