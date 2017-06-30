package internal

import (
	"net"

	"github.com/miekg/dns"
)

func WrapWriterWithIntercept(child dns.ResponseWriter, intercept func(m *dns.Msg)) dns.ResponseWriter {
	return &respWriterWrapperFunc{
		writeMsgFunc: intercept,
		child:        child,
	}
}

type respWriterWrapperFunc struct {
	writeMsgFunc func(m *dns.Msg)
	child        dns.ResponseWriter
}

func (r *respWriterWrapperFunc) WriteMsg(m *dns.Msg) error {
	r.writeMsgFunc(m)
	return r.child.WriteMsg(m)
}

func (r *respWriterWrapperFunc) Write(b []byte) (int, error) { panic("not implemented, use WriteMsg") }

func (r *respWriterWrapperFunc) LocalAddr() net.Addr   { return r.child.LocalAddr() }
func (r *respWriterWrapperFunc) RemoteAddr() net.Addr  { return r.child.RemoteAddr() }
func (r *respWriterWrapperFunc) Close() error          { return r.child.Close() }
func (r *respWriterWrapperFunc) TsigStatus() error     { return r.child.TsigStatus() }
func (r *respWriterWrapperFunc) TsigTimersOnly(b bool) { r.child.TsigTimersOnly(b) }
func (r *respWriterWrapperFunc) Hijack()               { r.child.Hijack() }
