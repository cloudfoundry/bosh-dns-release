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
