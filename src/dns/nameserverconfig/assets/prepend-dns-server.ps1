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
