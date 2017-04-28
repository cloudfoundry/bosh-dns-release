. .\assets\session.ps1

Invoke-Command -Session $session -ScriptBlock { netstat -na | findstr :53 }
