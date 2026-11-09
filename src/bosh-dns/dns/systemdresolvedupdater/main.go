package main

import (
	"flag"
	"log"
	"net"
	"path/filepath"
	"strings"
	"time"

	"bosh-dns/dns/config"
	"bosh-dns/dns/config/handlers"
	"bosh-dns/dns/manager"
	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/records"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/miekg/dns"
)

const logTag = "SystemdResolvedUpdater"

// maxLinkDomains is the most domains systemd-resolved accepts on one link.
const maxLinkDomains = 256

var systemdResolvedStubs = []net.IP{net.IPv4(127, 0, 0, 53), net.IPv4(127, 0, 0, 54)}

// main loads the bosh-dns configuration, the records, and the handlers, and
// then updates the systemd-resolved domains. It exits with a fatal log when
// any step fails.
func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	boshDnsConfig, err := config.LoadFromFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	level, err := boshDnsConfig.GetLogLevel()
	if err != nil {
		log.Fatal(err)
	}
	logr := logger.NewLogger(level)

	shutdown := make(chan struct{})
	fileReader := records.NewFileReader(boshDnsConfig.RecordsFile, system.NewOsFileSystem(logr), clock.NewClock(), logr, shutdown)
	fs := system.NewOsFileSystem(logr)
	healthWatcher := healthiness.NewNopHealthWatcher()

	aliasConfiguration, err := aliases.ConfigFromGlob(
		fs,
		aliases.NewFSLoader(fs),
		boshDnsConfig.AliasFilesGlob,
	)
	if err != nil {
		log.Fatal(err)
	}

	filtererFactory := records.NewHealthFiltererFactory(healthWatcher, time.Duration(boshDnsConfig.Health.SynchronousCheckTimeout))

	recordSet, err := records.NewRecordSet(fileReader, aliasConfiguration, healthWatcher, uint(boshDnsConfig.Health.MaxTrackedQueries), shutdown, logr, filtererFactory, records.NewAliasEncoder())
	if err != nil {
		log.Fatal(err)
	}

	handlersConfiguration, err := handlers.ConfigFromGlob(
		fs,
		handlers.NewFSLoader(fs),
		boshDnsConfig.HandlersFilesGlob,
	)
	if err != nil {
		log.Fatal(err)
	}

	systemdResolvedManager := manager.NewSystemdResolvedManager(system.NewExecCmdRunner(logr))
	err = updateSystemdResolved(systemdResolvedManager, boshDnsConfig, ownAliasSuffixFile(configPath), handlersConfiguration, recordSet, fs, logr)
	if err != nil {
		log.Fatal(err)
	}
	close(shutdown)
}

// updateSystemdResolved sends the domains for this configuration to
// systemd-resolved, and does nothing when systemd-resolved is not configured.
// It warns when there are more distinct domains than one link accepts, and
// still sends them. It returns the error from the manager.
func updateSystemdResolved(systemdResolvedManager manager.SystemdResolvedManager, cfg config.Config, ownSuffixFile string, handlersConfiguration handlers.HandlerConfigs, recordSet *records.RecordSet, fs system.FileSystem, logr logger.Logger) error {
	if !cfg.ConfigureSystemdResolved {
		return nil
	}

	domains := domainsFromConfig(cfg, ownSuffixFile, handlersConfiguration.HandlerDomains(), recordSet, fs, logr)

	distinct := map[string]bool{}
	for _, domain := range domains {
		distinct[dns.CanonicalName(domain)] = true
	}
	if len(distinct) > maxLinkDomains {
		logr.Warn(logTag, "Sending %d domains to systemd-resolved, which accepts at most %d on one link, so resolvectl is expected to fail. Set systemd_resolved.routing_domains to register a shared parent domain in place of the names under it.", len(distinct), maxLinkDomains)
	}

	return systemdResolvedManager.UpdateDomains(domains)
}

// ownAliasSuffixFile returns the path of this job's alias_suffixes.json,
// which sits in the dns directory beside the directory that holds the config.
func ownAliasSuffixFile(configPath string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(configPath)), "dns", "alias_suffixes.json")
}

// aliasSuffixFilesGlob returns the configured alias suffix files glob. When
// none is configured, it returns the default under the jobs directory, or an
// empty glob if there is no jobs directory.
func aliasSuffixFilesGlob(cfg config.Config) string {
	if cfg.AliasSuffixFilesGlob != "" || cfg.JobsDir == "" {
		return cfg.AliasSuffixFilesGlob
	}
	return filepath.Join(cfg.JobsDir, "*", "dns", "alias_suffixes.json")
}

// domainsFromConfig logs and skips suffixes and routing domains that cannot
// be used. It ignores all of them when bosh-dns can forward to the
// systemd-resolved stub resolver, because a name that bosh-dns cannot answer
// would then loop between the two.
func domainsFromConfig(cfg config.Config, ownSuffixFile string, handlerDomains []string, recordSet *records.RecordSet, fs system.FileSystem, logr logger.Logger) []string {
	var routingDomains []string
	for _, rawRoutingDomain := range cfg.SystemdResolvedRoutingDomains {
		routingDomain, err := aliases.CanonicalDomain(rawRoutingDomain)
		if err != nil {
			logr.Warn(logTag, "Ignoring the systemd_resolved.routing_domains entry %q: %s", rawRoutingDomain, err)
			continue
		}
		routingDomains = append(routingDomains, routingDomain)
	}

	suffixes := aliases.SuffixesFromGlob(fs, aliases.NewFSSuffixesLoader(fs), ownSuffixFile, aliasSuffixFilesGlob(cfg), logr)

	domains, registered := systemdResolvedDomains(handlerDomains, recordSet, suffixes, cfg.UpcheckDomains, routingDomains)
	if !forwardsToStubResolver(cfg) {
		return domains
	}

	if registered {
		logr.Warn(logTag, "Ignoring alias suffixes and systemd_resolved.routing_domains because bosh-dns can forward to the systemd-resolved stub resolver, which would send a name in those domains that bosh-dns cannot answer back to bosh-dns. Set disable_recursors: true, or set recursors that do not include 127.0.0.53 or 127.0.0.54.")
	}
	domains, _ = systemdResolvedDomains(handlerDomains, recordSet, aliases.NewSuffixes(), cfg.UpcheckDomains, nil)

	return domains
}

// forwardsToStubResolver treats empty recursors as the stub resolver,
// because bosh-dns then reads them from /etc/resolv.conf.
func forwardsToStubResolver(cfg config.Config) bool {
	if !cfg.ConfigureSystemdResolved || cfg.DisableRecursors {
		return false
	}

	recursors := cfg.Recursors
	if len(recursors) == 0 {
		recursors = []string{systemdResolvedStubs[0].String()}
	}

	excluded := map[string]bool{}
	for _, recursor := range cfg.ExcludedRecursors {
		if stub, ok := stubResolver(recursor); ok {
			excluded[stub] = true
		}
	}

	for _, recursor := range recursors {
		if stub, ok := stubResolver(recursor); ok && !excluded[stub] {
			return true
		}
	}

	return false
}

// stubResolver reports which systemd-resolved stub address a recursor names,
// where the recursor is an address with an optional port that defaults to 53.
// It returns false for any other address or port.
func stubResolver(recursor string) (string, bool) {
	host, port, err := net.SplitHostPort(recursor)
	if err != nil {
		host, port = recursor, "53"
	}

	ip := net.ParseIP(host)
	if ip == nil || port != "53" {
		return "", false
	}

	for _, stub := range systemdResolvedStubs {
		if ip.Equal(stub) {
			return stub.String(), true
		}
	}

	return "", false
}

// systemdResolvedDomains replaces each name with the longest routing domain
// it equals or sits under, and drops duplicates. The bool reports whether a
// suffix or a routing domain changed anything.
func systemdResolvedDomains(handlerDomains []string, recordSet *records.RecordSet, suffixes aliases.Suffixes, upcheckDomains []string, routingDomains []string) ([]string, bool) {
	var domains []string

	recordSetDomains, suffixed := recordSet.SystemdResolvedDomains(suffixes)

	domains = append(domains, handlerDomains...)
	domains = append(domains, recordSetDomains...)
	domains = append(domains, parentDomains(upcheckDomains)...)

	if len(routingDomains) == 0 && !suffixed {
		return domains, false
	}

	registered := suffixed
	seen := map[string]bool{}
	var uniqueDomains []string
	for _, domain := range domains {
		domain, matched := routingDomainFor(domain, routingDomains)
		registered = registered || matched

		if seen[domain] {
			continue
		}

		seen[domain] = true
		uniqueDomains = append(uniqueDomains, domain)
	}

	return uniqueDomains, registered
}

// routingDomainFor returns the routing domain with the most labels that the
// domain equals or sits under. It returns the domain unchanged and false when
// no routing domain applies.
func routingDomainFor(domain string, routingDomains []string) (string, bool) {
	name := dns.CanonicalName(domain)
	match := domain
	matchLabels := 0

	for _, routingDomain := range routingDomains {
		labels := dns.CountLabel(routingDomain)
		if labels > matchLabels && dns.IsSubDomain(routingDomain, name) {
			match = routingDomain
			matchLabels = labels
		}
	}

	return match, matchLabels > 0
}

func parentDomains(fqdns []string) []string {
	var parents []string
	for _, fqdn := range fqdns {
		if dotIdx := strings.Index(fqdn, "."); dotIdx >= 0 && dotIdx+1 < len(fqdn) {
			parents = append(parents, fqdn[dotIdx+1:])
		}
	}
	return parents
}
