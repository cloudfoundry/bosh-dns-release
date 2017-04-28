$passwd = convertto-securestring -AsPlainText -Force -String "${WINDOWS_DNS_SERVER_PASSWORD}"
$cred = new-object -typename System.Management.Automation.PSCredential -argumentlist "${WINDOWS_DNS_SERVER_HOST_NAME}\Administrator",$passwd
$session = new-pssession -computername $WINDOWS_DNS_SERVER_IP -credential $cred
