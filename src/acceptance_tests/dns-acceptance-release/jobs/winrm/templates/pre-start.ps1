try
{
    Enable-PSRemoting -Force
}
catch
{
    $Host.UI.WriteErrorLine($_.Exception.Message)
    Exit 1
}
Exit 0
