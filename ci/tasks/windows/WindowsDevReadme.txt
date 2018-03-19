Testing windows is painful. To make it less painful, you can use the existing test-acceptance scripts for launching
windows. To do this, don't bbl-destroy in `bin/test-acceptance-windows.sh` (if you are running tests and they fail, the
bin/test-acceptance-windows.sh` code will never hit the bbl-destroy). After the `fly execute` runs, `fly hijack` into
the fly container that launched the bosh director that was used to deploy the windows tests, and then use the
`bosh deploy` commands in test-acceptance-windows.sh to redeploy your Windows boxen.

Need to delete `bbl-state/creds.yml` in between runs to re-generate BOSH director CA cert for new IP addresses