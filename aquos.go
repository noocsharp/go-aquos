// Package aquos provides a client to connect to AQUOS.
package aquos

import (
	"bufio"
	"errors"
	"fmt"
	"log"
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
	Address     string
	LoginTimeout time.Duration

	conn net.Conn
	w    *bufio.Writer
	res  chan response
}

type response struct {
	text string
	err  error
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
			err := s.Err()
			c.res <- response{
				err: err,
			}
			log.Print(err)
			fmt.Println("got here\n");
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

func (c *Client) sendCommand(cmd, arg string) (string, error) {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	conn, err := dialer.Dial("tcp", c.Address)
	if err != nil {
		return "", err
	}
	c.conn = conn
	defer c.conn.Close()

	c.w = bufio.NewWriter(conn)

	c.res = make(chan response)
	go c.readLoop()

	if len(c.Username) != 0 && len(c.Password) != 0 {
		err = c.login()
		if err != nil {
			return "", err
		}
	}

	err = c.send(fmt.Sprintf("%s%-4s", cmd, arg))
	if err != nil {
		return "", err
	}
	res, err := c.readLine()
	if err != nil {
		return "", err
	}
	if res == "ERR" {
		err = errors.New("aquos returns a error")
		return "", err
	}

	return res, nil
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

func (c *Client) Play() error {
	_, err := c.sendCommand("RCKY", "16")
	return err
}

func (c *Client) FastForward() error {
	_, err := c.sendCommand("RCKY", "17")
	return err
}

func (c *Client) Pause() error {
	_, err := c.sendCommand("RCKY", "18")
	return err
}

func (c *Client) SkipBack() error {
	_, err := c.sendCommand("RCKY", "19")
	return err
}

func (c *Client) Stop() error {
	_, err := c.sendCommand("RCKY", "20")
	return err
}

func (c *Client) SkipForward() error {
	_, err := c.sendCommand("RCKY", "21")
	return err
}

func (c *Client) MuteToggle() error {
	_, err := c.sendCommand("MUTE", "0")
	return err
}

func (c *Client) VolumeDown() error {
	_, err := c.sendCommand("RCKY", "32")
	return err
}

func (c *Client) VolumeUp() error {
	_, err := c.sendCommand("RCKY", "33")
	return err
}

func (c *Client) Enter() error {
	_, err := c.sendCommand("RCKY", "40")
	return err
}

func (c *Client) Up() error {
	_, err := c.sendCommand("RCKY", "41")
	return err
}

func (c *Client) Down() error {
	_, err := c.sendCommand("RCKY", "42")
	return err
}

func (c *Client) Left() error {
	_, err := c.sendCommand("RCKY", "43")
	return err
}

func (c *Client) Right() error {
	_, err := c.sendCommand("RCKY", "44")
	return err
}

func (c *Client) Return() error {
	_, err := c.sendCommand("RCKY", "45")
	return err
}

func (c *Client) Exit() error {
	_, err := c.sendCommand("RCKY", "46")
	return err
}

func (c *Client) Netflix() error {
	_, err := c.sendCommand("RCKY", "59")
	return err
}
