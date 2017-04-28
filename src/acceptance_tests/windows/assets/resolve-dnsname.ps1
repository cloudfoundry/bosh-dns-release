. .\assets\session.ps1

Invoke-Command -Session $session -ScriptBlock { Resolve-DnsName -DnsOnly -Name healthcheck.bosh-dns. -Server 169.254.0.2 }
