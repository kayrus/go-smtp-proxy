package proxy

import (
	"io"

	"github.com/emersion/go-smtp"
)

type Session interface {
	smtp.Session
	Noop() error
	Status() *smtp.SMTPError
}

type session struct {
	c  *smtp.Client
	be *Backend
	st *smtp.SMTPError
}

func (s *session) Reset() {
	s.c.Reset()
}

func (s *session) Noop() error {
	return s.c.Noop()
}

func (s *session) Mail(from string, opts *smtp.MailOptions) error {
	return s.c.Mail(from, opts)
}

func (s *session) Rcpt(to string) error {
	return s.c.Rcpt(to)
}

func (s *session) Data(r io.Reader) error {
	wc, err := s.c.Data(s.statusCb)
	if err != nil {
		return err
	}

	_, err = io.Copy(wc, r)
	if err != nil {
		wc.Close()
		return err
	}

	return wc.Close()
}

func (s *session) Logout() error {
	return s.c.Quit()
}

func (s *session) Status() *smtp.SMTPError {
	return s.st
}

func (s *session) statusCb(status *smtp.SMTPError) {
	s.st = status
}
