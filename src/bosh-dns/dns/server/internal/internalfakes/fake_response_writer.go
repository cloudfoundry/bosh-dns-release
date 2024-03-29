// Code generated by counterfeiter. DO NOT EDIT.
package internalfakes

import (
	"net"
	"sync"

	"github.com/miekg/dns"
)

type FakeResponseWriter struct {
	CloseStub        func() error
	closeMutex       sync.RWMutex
	closeArgsForCall []struct {
	}
	closeReturns struct {
		result1 error
	}
	closeReturnsOnCall map[int]struct {
		result1 error
	}
	HijackStub        func()
	hijackMutex       sync.RWMutex
	hijackArgsForCall []struct {
	}
	LocalAddrStub        func() net.Addr
	localAddrMutex       sync.RWMutex
	localAddrArgsForCall []struct {
	}
	localAddrReturns struct {
		result1 net.Addr
	}
	localAddrReturnsOnCall map[int]struct {
		result1 net.Addr
	}
	RemoteAddrStub        func() net.Addr
	remoteAddrMutex       sync.RWMutex
	remoteAddrArgsForCall []struct {
	}
	remoteAddrReturns struct {
		result1 net.Addr
	}
	remoteAddrReturnsOnCall map[int]struct {
		result1 net.Addr
	}
	TsigStatusStub        func() error
	tsigStatusMutex       sync.RWMutex
	tsigStatusArgsForCall []struct {
	}
	tsigStatusReturns struct {
		result1 error
	}
	tsigStatusReturnsOnCall map[int]struct {
		result1 error
	}
	TsigTimersOnlyStub        func(bool)
	tsigTimersOnlyMutex       sync.RWMutex
	tsigTimersOnlyArgsForCall []struct {
		arg1 bool
	}
	WriteStub        func([]byte) (int, error)
	writeMutex       sync.RWMutex
	writeArgsForCall []struct {
		arg1 []byte
	}
	writeReturns struct {
		result1 int
		result2 error
	}
	writeReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	WriteMsgStub        func(*dns.Msg) error
	writeMsgMutex       sync.RWMutex
	writeMsgArgsForCall []struct {
		arg1 *dns.Msg
	}
	writeMsgReturns struct {
		result1 error
	}
	writeMsgReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeResponseWriter) Close() error {
	fake.closeMutex.Lock()
	ret, specificReturn := fake.closeReturnsOnCall[len(fake.closeArgsForCall)]
	fake.closeArgsForCall = append(fake.closeArgsForCall, struct {
	}{})
	stub := fake.CloseStub
	fakeReturns := fake.closeReturns
	fake.recordInvocation("Close", []interface{}{})
	fake.closeMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResponseWriter) CloseCallCount() int {
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	return len(fake.closeArgsForCall)
}

func (fake *FakeResponseWriter) CloseCalls(stub func() error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = stub
}

func (fake *FakeResponseWriter) CloseReturns(result1 error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = nil
	fake.closeReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResponseWriter) CloseReturnsOnCall(i int, result1 error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = nil
	if fake.closeReturnsOnCall == nil {
		fake.closeReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.closeReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResponseWriter) Hijack() {
	fake.hijackMutex.Lock()
	fake.hijackArgsForCall = append(fake.hijackArgsForCall, struct {
	}{})
	stub := fake.HijackStub
	fake.recordInvocation("Hijack", []interface{}{})
	fake.hijackMutex.Unlock()
	if stub != nil {
		fake.HijackStub()
	}
}

func (fake *FakeResponseWriter) HijackCallCount() int {
	fake.hijackMutex.RLock()
	defer fake.hijackMutex.RUnlock()
	return len(fake.hijackArgsForCall)
}

func (fake *FakeResponseWriter) HijackCalls(stub func()) {
	fake.hijackMutex.Lock()
	defer fake.hijackMutex.Unlock()
	fake.HijackStub = stub
}

func (fake *FakeResponseWriter) LocalAddr() net.Addr {
	fake.localAddrMutex.Lock()
	ret, specificReturn := fake.localAddrReturnsOnCall[len(fake.localAddrArgsForCall)]
	fake.localAddrArgsForCall = append(fake.localAddrArgsForCall, struct {
	}{})
	stub := fake.LocalAddrStub
	fakeReturns := fake.localAddrReturns
	fake.recordInvocation("LocalAddr", []interface{}{})
	fake.localAddrMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResponseWriter) LocalAddrCallCount() int {
	fake.localAddrMutex.RLock()
	defer fake.localAddrMutex.RUnlock()
	return len(fake.localAddrArgsForCall)
}

func (fake *FakeResponseWriter) LocalAddrCalls(stub func() net.Addr) {
	fake.localAddrMutex.Lock()
	defer fake.localAddrMutex.Unlock()
	fake.LocalAddrStub = stub
}

func (fake *FakeResponseWriter) LocalAddrReturns(result1 net.Addr) {
	fake.localAddrMutex.Lock()
	defer fake.localAddrMutex.Unlock()
	fake.LocalAddrStub = nil
	fake.localAddrReturns = struct {
		result1 net.Addr
	}{result1}
}

func (fake *FakeResponseWriter) LocalAddrReturnsOnCall(i int, result1 net.Addr) {
	fake.localAddrMutex.Lock()
	defer fake.localAddrMutex.Unlock()
	fake.LocalAddrStub = nil
	if fake.localAddrReturnsOnCall == nil {
		fake.localAddrReturnsOnCall = make(map[int]struct {
			result1 net.Addr
		})
	}
	fake.localAddrReturnsOnCall[i] = struct {
		result1 net.Addr
	}{result1}
}

func (fake *FakeResponseWriter) RemoteAddr() net.Addr {
	fake.remoteAddrMutex.Lock()
	ret, specificReturn := fake.remoteAddrReturnsOnCall[len(fake.remoteAddrArgsForCall)]
	fake.remoteAddrArgsForCall = append(fake.remoteAddrArgsForCall, struct {
	}{})
	stub := fake.RemoteAddrStub
	fakeReturns := fake.remoteAddrReturns
	fake.recordInvocation("RemoteAddr", []interface{}{})
	fake.remoteAddrMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResponseWriter) RemoteAddrCallCount() int {
	fake.remoteAddrMutex.RLock()
	defer fake.remoteAddrMutex.RUnlock()
	return len(fake.remoteAddrArgsForCall)
}

func (fake *FakeResponseWriter) RemoteAddrCalls(stub func() net.Addr) {
	fake.remoteAddrMutex.Lock()
	defer fake.remoteAddrMutex.Unlock()
	fake.RemoteAddrStub = stub
}

func (fake *FakeResponseWriter) RemoteAddrReturns(result1 net.Addr) {
	fake.remoteAddrMutex.Lock()
	defer fake.remoteAddrMutex.Unlock()
	fake.RemoteAddrStub = nil
	fake.remoteAddrReturns = struct {
		result1 net.Addr
	}{result1}
}

func (fake *FakeResponseWriter) RemoteAddrReturnsOnCall(i int, result1 net.Addr) {
	fake.remoteAddrMutex.Lock()
	defer fake.remoteAddrMutex.Unlock()
	fake.RemoteAddrStub = nil
	if fake.remoteAddrReturnsOnCall == nil {
		fake.remoteAddrReturnsOnCall = make(map[int]struct {
			result1 net.Addr
		})
	}
	fake.remoteAddrReturnsOnCall[i] = struct {
		result1 net.Addr
	}{result1}
}

func (fake *FakeResponseWriter) TsigStatus() error {
	fake.tsigStatusMutex.Lock()
	ret, specificReturn := fake.tsigStatusReturnsOnCall[len(fake.tsigStatusArgsForCall)]
	fake.tsigStatusArgsForCall = append(fake.tsigStatusArgsForCall, struct {
	}{})
	stub := fake.TsigStatusStub
	fakeReturns := fake.tsigStatusReturns
	fake.recordInvocation("TsigStatus", []interface{}{})
	fake.tsigStatusMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResponseWriter) TsigStatusCallCount() int {
	fake.tsigStatusMutex.RLock()
	defer fake.tsigStatusMutex.RUnlock()
	return len(fake.tsigStatusArgsForCall)
}

func (fake *FakeResponseWriter) TsigStatusCalls(stub func() error) {
	fake.tsigStatusMutex.Lock()
	defer fake.tsigStatusMutex.Unlock()
	fake.TsigStatusStub = stub
}

func (fake *FakeResponseWriter) TsigStatusReturns(result1 error) {
	fake.tsigStatusMutex.Lock()
	defer fake.tsigStatusMutex.Unlock()
	fake.TsigStatusStub = nil
	fake.tsigStatusReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResponseWriter) TsigStatusReturnsOnCall(i int, result1 error) {
	fake.tsigStatusMutex.Lock()
	defer fake.tsigStatusMutex.Unlock()
	fake.TsigStatusStub = nil
	if fake.tsigStatusReturnsOnCall == nil {
		fake.tsigStatusReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.tsigStatusReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResponseWriter) TsigTimersOnly(arg1 bool) {
	fake.tsigTimersOnlyMutex.Lock()
	fake.tsigTimersOnlyArgsForCall = append(fake.tsigTimersOnlyArgsForCall, struct {
		arg1 bool
	}{arg1})
	stub := fake.TsigTimersOnlyStub
	fake.recordInvocation("TsigTimersOnly", []interface{}{arg1})
	fake.tsigTimersOnlyMutex.Unlock()
	if stub != nil {
		fake.TsigTimersOnlyStub(arg1)
	}
}

func (fake *FakeResponseWriter) TsigTimersOnlyCallCount() int {
	fake.tsigTimersOnlyMutex.RLock()
	defer fake.tsigTimersOnlyMutex.RUnlock()
	return len(fake.tsigTimersOnlyArgsForCall)
}

func (fake *FakeResponseWriter) TsigTimersOnlyCalls(stub func(bool)) {
	fake.tsigTimersOnlyMutex.Lock()
	defer fake.tsigTimersOnlyMutex.Unlock()
	fake.TsigTimersOnlyStub = stub
}

func (fake *FakeResponseWriter) TsigTimersOnlyArgsForCall(i int) bool {
	fake.tsigTimersOnlyMutex.RLock()
	defer fake.tsigTimersOnlyMutex.RUnlock()
	argsForCall := fake.tsigTimersOnlyArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeResponseWriter) Write(arg1 []byte) (int, error) {
	var arg1Copy []byte
	if arg1 != nil {
		arg1Copy = make([]byte, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.writeMutex.Lock()
	ret, specificReturn := fake.writeReturnsOnCall[len(fake.writeArgsForCall)]
	fake.writeArgsForCall = append(fake.writeArgsForCall, struct {
		arg1 []byte
	}{arg1Copy})
	stub := fake.WriteStub
	fakeReturns := fake.writeReturns
	fake.recordInvocation("Write", []interface{}{arg1Copy})
	fake.writeMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeResponseWriter) WriteCallCount() int {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	return len(fake.writeArgsForCall)
}

func (fake *FakeResponseWriter) WriteCalls(stub func([]byte) (int, error)) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = stub
}

func (fake *FakeResponseWriter) WriteArgsForCall(i int) []byte {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	argsForCall := fake.writeArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeResponseWriter) WriteReturns(result1 int, result2 error) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = nil
	fake.writeReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakeResponseWriter) WriteReturnsOnCall(i int, result1 int, result2 error) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = nil
	if fake.writeReturnsOnCall == nil {
		fake.writeReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.writeReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakeResponseWriter) WriteMsg(arg1 *dns.Msg) error {
	fake.writeMsgMutex.Lock()
	ret, specificReturn := fake.writeMsgReturnsOnCall[len(fake.writeMsgArgsForCall)]
	fake.writeMsgArgsForCall = append(fake.writeMsgArgsForCall, struct {
		arg1 *dns.Msg
	}{arg1})
	stub := fake.WriteMsgStub
	fakeReturns := fake.writeMsgReturns
	fake.recordInvocation("WriteMsg", []interface{}{arg1})
	fake.writeMsgMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResponseWriter) WriteMsgCallCount() int {
	fake.writeMsgMutex.RLock()
	defer fake.writeMsgMutex.RUnlock()
	return len(fake.writeMsgArgsForCall)
}

func (fake *FakeResponseWriter) WriteMsgCalls(stub func(*dns.Msg) error) {
	fake.writeMsgMutex.Lock()
	defer fake.writeMsgMutex.Unlock()
	fake.WriteMsgStub = stub
}

func (fake *FakeResponseWriter) WriteMsgArgsForCall(i int) *dns.Msg {
	fake.writeMsgMutex.RLock()
	defer fake.writeMsgMutex.RUnlock()
	argsForCall := fake.writeMsgArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeResponseWriter) WriteMsgReturns(result1 error) {
	fake.writeMsgMutex.Lock()
	defer fake.writeMsgMutex.Unlock()
	fake.WriteMsgStub = nil
	fake.writeMsgReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResponseWriter) WriteMsgReturnsOnCall(i int, result1 error) {
	fake.writeMsgMutex.Lock()
	defer fake.writeMsgMutex.Unlock()
	fake.WriteMsgStub = nil
	if fake.writeMsgReturnsOnCall == nil {
		fake.writeMsgReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.writeMsgReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResponseWriter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	fake.hijackMutex.RLock()
	defer fake.hijackMutex.RUnlock()
	fake.localAddrMutex.RLock()
	defer fake.localAddrMutex.RUnlock()
	fake.remoteAddrMutex.RLock()
	defer fake.remoteAddrMutex.RUnlock()
	fake.tsigStatusMutex.RLock()
	defer fake.tsigStatusMutex.RUnlock()
	fake.tsigTimersOnlyMutex.RLock()
	defer fake.tsigTimersOnlyMutex.RUnlock()
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	fake.writeMsgMutex.RLock()
	defer fake.writeMsgMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeResponseWriter) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}
