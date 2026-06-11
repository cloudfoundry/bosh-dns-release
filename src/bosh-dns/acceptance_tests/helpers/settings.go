package helpers

// OverrideNameserverFor reports whether bosh-dns should manage /etc/resolv.conf
// for the given stemcell OS, mirroring the override_nameserver release property.
func OverrideNameserverFor(stemcell string) bool {
	// TODO: remove when Jammy goes EOL
	return stemcell == "ubuntu-jammy"
}

// ConfigureSystemdResolvedFor reports whether bosh-dns should configure
// systemd-resolved for the given stemcell OS, mirroring the
// configure_systemd_resolved release property.
func ConfigureSystemdResolvedFor(stemcell string) bool {
	// TODO: remove when Jammy goes EOL
	return stemcell != "ubuntu-jammy"
}
