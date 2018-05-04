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
			UnicastAddresses:   ipsFromSocketAddresses(socketAddressesFromIpAdapterUnicastAddress(aa.FirstUnicastAddress)),
			DNSServerAddresses: ipsFromSocketAddresses(socketAddressesFromIpAdapterDnsServerAdapter(aa.FirstDnsServerAddress)),
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

func ipsFromSocketAddresses(addresses []windows.SocketAddress) []string {
	var results []string

	for _, address := range addresses {
		ipAddr, err := sockaddrToIP(address)
		if err != nil {
			// error parsing socket address to IP
			continue
		}

		results = append(results, ipAddr)
	}

	return results
}

func socketAddressesFromIpAdapterUnicastAddress(addresses *windows.IpAdapterUnicastAddress) []windows.SocketAddress {
	var results []windows.SocketAddress

	for address := addresses; address != nil; address = address.Next {
		results = append(results, address.Address)
	}

	return results
}

func socketAddressesFromIpAdapterDnsServerAdapter(addresses *windows.IpAdapterDnsServerAdapter) []windows.SocketAddress {
	var results []windows.SocketAddress

	for address := addresses; address != nil; address = address.Next {
		results = append(results, address.Address)
	}

	return results
}
