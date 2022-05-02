package proxy

import (
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

type Security int

const (
	SecurityTLS Security = iota
	SecurityStartTLS
	SecurityNone
)

var (
	// 30 seconds was chosen as it's the
	// same duration as http.DefaultTransport's timeout.
	defaultTimeout = 30 * time.Second
	// As recommended by RFC 5321. For DATA command reply (3xx one) RFC
	// recommends a slightly shorter timeout but we do not bother
	// differentiating these.
	defaultCommandTimeout = 5 * time.Minute
	// 10 minutes + 2 minute buffer in case the server is doing transparent
	// forwarding and also follows recommended timeouts.
	defaultSubmissionTimeout = 12 * time.Minute
)

type Backend struct {
	Addr              string
	Security          Security
	TLSConfig         *tls.Config
	LMTP              bool
	Host              string
	LocalName         string
	DialTimeout       time.Duration
	CommandTimeout    time.Duration
	SubmissionTimeout time.Duration

	unexported struct{}
}

func New(addr string) *Backend {
	return &Backend{Addr: addr, Security: SecurityStartTLS}
}

func NewTLS(addr string, tlsConfig *tls.Config) *Backend {
	return &Backend{
		Addr:      addr,
		Security:  SecurityTLS,
		TLSConfig: tlsConfig,
	}
}

func NewLMTP(addr string, host string) *Backend {
	return &Backend{
		Addr:     addr,
		Security: SecurityNone,
		LMTP:     true,
		Host:     host,
	}
}

func (be *Backend) newConn() (*smtp.Client, error) {
	var conn net.Conn
	var err error

	dialTimeout := be.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = defaultTimeout
	}
	commandTimeout := be.CommandTimeout
	if commandTimeout == 0 {
		commandTimeout = defaultCommandTimeout
	}
	submissionTimeout := be.SubmissionTimeout
	if submissionTimeout == 0 {
		submissionTimeout = defaultSubmissionTimeout
	}

	host := be.Host
	if host == "" {
		host, _, _ = net.SplitHostPort(be.Addr)
	}
	var tlsConfig *tls.Config
	if be.TLSConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = be.TLSConfig.Clone()
	}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = host
	}

	if be.LMTP {
		if be.Security != SecurityNone {
			return nil, errors.New("smtp-proxy: LMTP doesn't support TLS")
		}
		conn, err = net.DialTimeout("tcp", be.Addr, dialTimeout)
	} else if be.Security == SecurityTLS {
		tlsDialer := tls.Dialer{
			NetDialer: &net.Dialer{
				Timeout: dialTimeout,
			},
			Config: tlsConfig,
		}
		conn, err = tlsDialer.Dial("tcp", be.Addr)
	} else {
		conn, err = net.DialTimeout("tcp", be.Addr, dialTimeout)
	}
	if err != nil {
		return nil, err
	}

	var c *smtp.Client
	if be.LMTP {
		c, err = smtp.NewClientLMTP(conn, be.Host)
	} else {
		c = &smtp.Client{
			CommandTimeout:    commandTimeout,
			SubmissionTimeout: submissionTimeout,
		}
		err = c.InitConn(conn)
	}
	if err != nil {
		return c, err
	}

	if be.LocalName != "" {
		err = c.Hello(be.LocalName)
		if err != nil {
			return c, err
		}
	}

	if be.Security == SecurityStartTLS {
		if err := c.StartTLS(tlsConfig); err != nil {
			return c, err
		}
	}

	return c, nil
}

func (be *Backend) login(username, password string) (*smtp.Client, error) {
	c, err := be.newConn()
	if err != nil {
		return c, err
	}

	auth := sasl.NewPlainClient("", username, password)
	if err := c.Auth(auth); err != nil {
		return c, err
	}

	return c, nil
}

func (be *Backend) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	c, err := be.login(username, password)
	if err != nil {
		if c != nil {
			c.Close()
		}
		return nil, err
	}

	s := &session{
		c:  c,
		be: be,
	}
	return s, nil
}

func (be *Backend) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	c, err := be.newConn()
	if err != nil {
		if c != nil {
			c.Close()
		}
		return nil, err
	}

	s := &session{
		c:  c,
		be: be,
	}
	return s, nil
}
