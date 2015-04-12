package hive

import (
	"bufio"
	"errors"
	"sync"
	"time"

	"github.com/gosexy/to"
	"github.com/tarm/serial"
	"github.com/xiam/resp"
)

var (
	ErrNilReply                = errors.New(`Received a nil response.`)
	ErrMaxReadAttemptsExceeded = errors.New(`Exceeded maximum read attempts.`)
)

var (
	defaultTimeout     = time.Second * 5
	defaultConnectWait = time.Second * 3

	maxReadAttempts     = 50
	maxReadAttemptsWait = 5
)

func toBytesArray(values ...interface{}) [][]byte {
	cvalues := make([][]byte, len(values))

	for i := 0; i < len(values); i++ {
		cvalues[i] = to.Bytes(values[i])
	}

	return cvalues
}

type Client struct {
	cfg  *serial.Config
	port *serial.Port

	mu sync.Mutex

	encoder *resp.Encoder
	decoder *resp.Decoder

	reader *bufio.Reader
	writer *bufio.Writer
}

func NewClient(port string, speed int) (*Client, error) {
	var err error

	c := new(Client)

	c.cfg = &serial.Config{
		Name:        port,
		Baud:        speed,
		ReadTimeout: defaultTimeout,
	}

	if c.port, err = serial.OpenPort(c.cfg); err != nil {
		return nil, err
	}

	c.reader = bufio.NewReader(c.port)
	c.writer = bufio.NewWriter(c.port)

	c.encoder = resp.NewEncoder(c.writer)
	c.decoder = resp.NewDecoder(c.reader)

	time.Sleep(defaultConnectWait)

	return c, nil
}

func (c *Client) Command(dest interface{}, arguments ...interface{}) error {
	var err error

	c.mu.Lock()
	defer c.mu.Unlock()

	command := toBytesArray(arguments...)

	if err = c.encoder.Encode(command); err != nil {
		return err
	}

	if err = c.writer.Flush(); err != nil {
		return err
	}

	for i := 0; ; i++ {
		err = c.decoder.Decode(dest)
		if err == resp.ErrInvalidInput {
			time.Sleep(time.Millisecond * time.Duration(i*maxReadAttemptsWait))
		} else {
			break
		}
		if i > maxReadAttempts {
			return ErrMaxReadAttemptsExceeded
		}
	}

	if dest == nil && err == resp.ErrExpectingDestination {
		return nil
	}

	if err == resp.ErrMessageIsNil {
		return ErrNilReply
	}

	return err
}
