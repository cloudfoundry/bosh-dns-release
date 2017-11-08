package manager

import (
	"path/filepath"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const listResolvers = `
$ErrorActionPreference = "Stop"

try {
  [array]$routeable_interfaces = Get-WmiObject Win32_NetworkAdapterConfiguration | Where { $_.IpAddress -AND ($_.IpAddress | Where { $addr = [Net.IPAddress] $_; $addr.AddressFamily -eq "InterNetwork" -AND ($addr.address -BAND ([Net.IPAddress] "255.255.0.0").address) -ne ([Net.IPAddress] "169.254.0.0").address }) }

  $interface = (Get-WmiObject Win32_NetworkAdapter | Where { $_.DeviceID -eq $routeable_interfaces[0].Index }).netconnectionid

  (Get-DnsClientServerAddress -InterfaceAlias $interface -AddressFamily ipv4 -ErrorAction Stop).ServerAddresses
} catch {
  $Host.UI.WriteErrorLine($_.Exception.Message)
  Exit 1
}
Exit 0
`

const prependDNSServer = `
param ($DNSAddress = $(throw "DNSAddress parameter is required."))

$ErrorActionPreference = "Stop"

function DnsServers($interface) {
  return (Get-DnsClientServerAddress -InterfaceAlias $interface -AddressFamily ipv4 -ErrorAction Stop).ServerAddresses
}

try {
  # identify our interface
  [array]$routeable_interfaces = Get-WmiObject Win32_NetworkAdapterConfiguration | Where { $_.IpAddress -AND ($_.IpAddress | Where { $addr = [Net.IPAddress] $_; $addr.AddressFamily -eq "InterNetwork" -AND ($addr.address -BAND ([Net.IPAddress] "255.255.0.0").address) -ne ([Net.IPAddress] "169.254.0.0").address }) }
  $interface = (Get-WmiObject Win32_NetworkAdapter | Where { $_.DeviceID -eq $routeable_interfaces[0].Index }).netconnectionid

  # avoid prepending if we happen to already be at the top to try and avoid races
  [array]$servers = DnsServers($interface)
  if($servers[0] -eq $DNSAddress) {
    Exit 0
  }

  Set-DnsClientServerAddress -InterfaceAlias $interface -ServerAddresses (,$DNSAddress + $servers)

  # read back the servers in case set silently failed
  [array]$servers = DnsServers($interface)
  if($servers[0] -ne $DNSAddress) {
      Write-Error "Failed to set '${DNSAddress}' as the first dns client server address"
  }
} catch {
  $Host.UI.WriteErrorLine($_.Exception.Message)
  Exit 1
}

Exit 0
`

type windowsManager struct {
	runner boshsys.CmdRunner
	fs     boshsys.FileSystem
}

func NewWindowsManager(runner boshsys.CmdRunner, fs boshsys.FileSystem) *windowsManager {
	return &windowsManager{runner: runner, fs: fs}
}

func (manager *windowsManager) SetPrimary(address string) error {
	servers, err := manager.Read()
	if err != nil {
		return err
	}

	if len(servers) > 0 && servers[0] == address {
		return nil
	}

	scriptName, err := manager.writeScript("prepend-dns-server", prependDNSServer)
	if err != nil {
		return bosherr.WrapError(err, "Creating prepend-dns-server.ps1")
	}
	defer manager.fs.RemoveAll(filepath.Dir(scriptName))

	_, _, _, err = manager.runner.RunCommand("powershell.exe", scriptName, address)
	if err != nil {
		return bosherr.WrapError(err, "Executing prepend-dns-server.ps1")
	}

	return nil
}

func (manager *windowsManager) Read() ([]string, error) {
	scriptName, err := manager.writeScript("list-dns-servers", listResolvers)
	if err != nil {
		return nil, bosherr.WrapError(err, "Creating list-server-addresses.ps1")
	}
	defer manager.fs.RemoveAll(filepath.Dir(scriptName))

	stdout, _, _, err := manager.runner.RunCommand("powershell.exe", scriptName)
	if err != nil {
		return nil, bosherr.WrapError(err, "Executing list-server-addresses.ps1")
	}

	servers := strings.Split(stdout, "\r\n")
	if servers[0] == "" {
		return []string{}, nil
	}

	return servers, nil
}

func (manager *windowsManager) writeScript(name, contents string) (string, error) {
	dir, err := manager.fs.TempDir(name)
	if err != nil {
		return "", err
	}

	scriptName := filepath.Join(dir, name+".ps1")
	err = manager.fs.WriteFileString(scriptName, contents)
	if err != nil {
		return "", err
	}

	err = manager.fs.Chmod(scriptName, 0700)
	if err != nil {
		return "", err
	}

	return scriptName, nil
}
