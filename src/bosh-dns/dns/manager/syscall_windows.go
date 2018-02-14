// +build windows

package manager

import (
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const hexDigit = "0123456789abcdef"

type WindowsAdapterFetcher struct {
}

func (WindowsAdapterFetcher) Adapters() ([]Adapter, error) {
	var b []byte
	l := uint32(15000) // recommended initial size
	for {
		b = make([]byte, l)
		err := windows.GetAdaptersAddresses(syscall.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return nil, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
		if l <= uint32(len(b)) {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
	}
	var aas []Adapter
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		aas = append(aas, Adapter{
			IfType:             aa.IfType,
			OperStatus:         aa.OperStatus,
			PhysicalAddress:    physicalAddrToString(aa.PhysicalAddress),
			DNSServerAddresses: toArray(aa.FirstDnsServerAddress),
		})
	}
	return aas, nil
}

func sockaddrToIP(sockaddr windows.SocketAddress) (string, error) {
	sa, err := sockaddr.Sockaddr.Sockaddr()
	if err != nil {
		return "", os.NewSyscallError("sockaddr", err)
	}

	switch sa := sa.(type) {
	case *syscall.SockaddrInet4:
		ifa := &net.IPAddr{IP: net.IPv4(sa.Addr[0], sa.Addr[1], sa.Addr[2], sa.Addr[3])}
		return ifa.String(), nil
	case *syscall.SockaddrInet6:
		ifa := &net.IPAddr{IP: make(net.IP, net.IPv6len)}
		copy(ifa.IP, sa.Addr[:])
		return ifa.String(), nil
	}

	return "", nil
}

func toArray(dnsAddress *windows.IpAdapterDnsServerAdapter) []string {
	var resolvers []string

	for aa := dnsAddress; aa != nil; aa = aa.Next {
		ipAddr, err := sockaddrToIP(aa.Address)
		if err != nil {
			// just like whatever => fmt.Printf("Error: %s", err)
			continue
		}
		resolvers = append(resolvers, ipAddr)
	}

	return resolvers
}

func physicalAddrToString(physAddr [8]byte) string {
	if len(physAddr) == 0 {
		return ""
	}
	buf := make([]byte, 0, len(physAddr)*3-1)
	for i, b := range physAddr {
		if i > 0 {
			buf = append(buf, ':')
		}
		buf = append(buf, hexDigit[b>>4])
		buf = append(buf, hexDigit[b&0xF])
	}
	return string(buf)
}
