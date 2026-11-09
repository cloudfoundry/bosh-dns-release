require_relative 'shared_examples'

def render_monit(properties = {})
  job_dir = File.join(File.dirname(__FILE__), '../../jobs/bosh-dns')
  spec = YAML.safe_load(File.read(File.join(job_dir, 'spec')))
  Bosh::Template::Test::Template.new(spec, File.join(job_dir, 'monit')).render(properties)
end

describe 'bosh-dns' do
  let(:release) { Bosh::Template::Test::ReleaseDir.new(File.join(File.dirname(__FILE__), '../..')) }
  let(:job) { release.job('bosh-dns') }

  it_behaves_like 'common config.json'

  describe 'config/config.json' do
    let(:template) { job.template('config/config.json') }
    let(:properties) { {} }
    let(:rendered) { JSON.parse(template.render(properties)) }
    let(:corpus) do
      JSON.parse(File.read(File.join(File.dirname(__FILE__), '../../src/bosh-dns/dns/server/aliases/testdata/domain_corpus.json')))
    end

    def render_error(properties)
      template.render(properties)
      nil
    rescue StandardError => e
      e.message
    end

    context 'with no properties' do
      it 'writes the same keys as without alias suffixes and routing domains' do
        expect(rendered.keys).to contain_exactly(
          'address', 'port', 'log_level', 'disable_recursors', 'configure_systemd_resolved', 'recursors',
          'excluded_recursors', 'records_file', 'addresses_files_glob', 'alias_files_glob', 'upcheck_domains',
          'recursor_timeout', 'recursor_max_retries', 'request_timeout', 'recursor_selection', 'jobs_dir', 'api',
          'health', 'metrics', 'cache', 'handlers_files_glob', 'internal_upcheck_domain', 'logging'
        )
      end
    end

    context 'systemd_resolved.routing_domains' do
      context 'configured' do
        let(:properties) { {'systemd_resolved' => {'routing_domains' => ['bosh', 'Service.CF.Internal.']}} }

        it 'writes the routing domains' do
          expect(rendered['systemd_resolved_routing_domains']).to eq(['bosh', 'Service.CF.Internal.'])
        end
      end

      it 'accepts and rejects the same domains as the systemd-resolved updater' do
        mismatches = corpus['domains'].reject do |entry|
          error = render_error('systemd_resolved' => {'routing_domains' => ['bosh', entry['domain']]})
          if entry['error'].empty?
            error.nil?
          else
            error&.start_with?("systemd_resolved.routing_domains entry #{entry['domain'].inspect}: #{entry['error']}")
          end
        end

        expect(mismatches).to be_empty
      end

      context 'with an entry that is not a string' do
        let(:properties) { {'systemd_resolved' => {'routing_domains' => [53]}} }

        it 'fails to render' do
          expect { rendered }.to raise_error(/systemd_resolved\.routing_domains entry 53: not a string/)
        end
      end

      context 'when it is not an array' do
        let(:properties) { {'systemd_resolved' => {'routing_domains' => 'bosh'}} }

        it 'fails to render' do
          expect { rendered }.to raise_error(/systemd_resolved\.routing_domains must be an array/)
        end
      end
    end

    context 'alias_suffix_files_glob' do
      context 'configured' do
        let(:properties) { {'alias_suffix_files_glob' => '/other/*/suffixes.json'} }

        it 'writes the glob' do
          expect(rendered['alias_suffix_files_glob']).to eq('/other/*/suffixes.json')
        end
      end
    end

    context 'alias_suffixes' do
      it 'accepts and rejects the same entries as the systemd-resolved updater' do
        mismatches = corpus['alias_suffixes'].reject do |entry|
          error = render_error('alias_suffixes' => {entry['alias'] => entry['suffix']})
          if entry['error'].empty?
            error.nil?
          else
            error&.start_with?("alias_suffixes entry #{entry['alias'].inspect}: ") && error.include?(entry['error'])
          end
        end

        expect(mismatches).to be_empty
      end

      context 'with a plain and an underscore alias on the same host' do
        let(:properties) { {'alias_suffixes' => {'a.b.internal' => 'b.internal', '_.a.b.internal' => 'internal'}} }

        it 'renders' do
          expect(rendered['configure_systemd_resolved']).to eq(false)
        end
      end

      context 'with keys that differ only in case and name the same suffix' do
        let(:properties) { {'alias_suffixes' => {'bbs.service.cf.internal' => 'service.cf.internal', 'BBS.Service.CF.Internal' => 'Service.CF.Internal.'}} }

        it 'renders' do
          expect(rendered['configure_systemd_resolved']).to eq(false)
        end
      end

      context 'with keys that differ only in case and name different suffixes' do
        let(:properties) do
          {'alias_suffixes' => {'bbs.service.cf.internal' => 'cf.internal', 'BBS.Service.CF.Internal' => 'service.cf.internal'}}
        end

        it 'fails to render' do
          expect { rendered }.to raise_error(/alias_suffixes entry "bbs\.service\.cf\.internal": the alias already has the suffix "service\.cf\.internal\."/)
        end
      end

      context 'with a suffix that is not a string' do
        let(:properties) { {'alias_suffixes' => {'bbs.service.cf.internal' => ['service.cf.internal']}} }

        it 'fails to render' do
          expect { rendered }.to raise_error(/alias_suffixes entry "bbs\.service\.cf\.internal": suffix \["service\.cf\.internal"\]: not a string/)
        end
      end

      context 'with an alias that is not a string' do
        let(:properties) { {'alias_suffixes' => {1 => 'internal'}} }

        it 'fails to render, because the template engine only accepts string property keys' do
          expect { rendered }.to raise_error(NoMethodError, /to_sym/)
        end
      end

      context 'when it is not a hash' do
        let(:properties) { {'alias_suffixes' => ['service.cf.internal']} }

        it 'fails to render' do
          expect { rendered }.to raise_error(/alias_suffixes must be a hash of alias to domain/)
        end
      end
    end

    context 'when configure_systemd_resolved is true and bosh-dns can forward to the systemd-resolved stub resolver' do
      let(:stub_properties) { {'configure_systemd_resolved' => true, 'disable_recursors' => false, 'recursors' => []} }

      context 'with routing domains' do
        let(:properties) { stub_properties.merge('systemd_resolved' => {'routing_domains' => ['bosh']}) }

        it 'fails to render and names the fix' do
          expect { rendered }.to raise_error(/need disable_recursors: true, or recursors that do not include the systemd-resolved stub resolver/)
        end
      end

      context 'with alias suffixes' do
        let(:properties) { stub_properties.merge('alias_suffixes' => {'bbs.service.cf.internal' => 'service.cf.internal'}) }

        it 'fails to render and names the fix' do
          expect { rendered }.to raise_error(/need disable_recursors: true/)
        end
      end

      context 'with the stub resolver as an explicit recursor' do
        let(:properties) { stub_properties.merge('recursors' => ['10.0.0.2', '127.0.0.53:53'], 'systemd_resolved' => {'routing_domains' => ['bosh']}) }

        it 'fails to render' do
          expect { rendered }.to raise_error(/need disable_recursors: true/)
        end
      end

      context 'with the proxy stub resolver as an explicit recursor' do
        let(:properties) { stub_properties.merge('recursors' => ['127.0.0.54'], 'systemd_resolved' => {'routing_domains' => ['bosh']}) }

        it 'fails to render' do
          expect { rendered }.to raise_error(/need disable_recursors: true/)
        end
      end

      context 'without routing domains or alias suffixes' do
        let(:properties) { stub_properties }

        it 'renders' do
          expect(rendered['configure_systemd_resolved']).to eq(true)
        end
      end
    end

    context 'when configure_systemd_resolved is true and bosh-dns cannot forward to the systemd-resolved stub resolver' do
      let(:stub_properties) do
        {
          'configure_systemd_resolved' => true,
          'disable_recursors' => false,
          'recursors' => [],
          'systemd_resolved' => {'routing_domains' => ['bosh']},
          'alias_suffixes' => {'bbs.service.cf.internal' => 'service.cf.internal'},
        }
      end

      context 'with disable_recursors set to true' do
        let(:properties) { stub_properties.merge('disable_recursors' => true) }

        it 'renders' do
          expect(rendered['systemd_resolved_routing_domains']).to eq(['bosh'])
        end
      end

      context 'with explicit recursors' do
        let(:properties) { stub_properties.merge('recursors' => ['10.0.0.2']) }

        it 'renders' do
          expect(rendered['systemd_resolved_routing_domains']).to eq(['bosh'])
        end
      end

      context 'with the stub resolver on another port' do
        let(:properties) { stub_properties.merge('recursors' => ['127.0.0.53:5353']) }

        it 'renders' do
          expect(rendered['systemd_resolved_routing_domains']).to eq(['bosh'])
        end
      end

      context 'with the stub resolver read from resolv.conf and excluded' do
        let(:properties) { stub_properties.merge('excluded_recursors' => ['127.0.0.53']) }

        it 'renders' do
          expect(rendered['systemd_resolved_routing_domains']).to eq(['bosh'])
        end
      end

      context 'with the stub resolver as an explicit recursor and excluded' do
        let(:properties) { stub_properties.merge('recursors' => ['127.0.0.53'], 'excluded_recursors' => ['127.0.0.53:53']) }

        it 'renders' do
          expect(rendered['systemd_resolved_routing_domains']).to eq(['bosh'])
        end
      end
    end

    context 'when configure_systemd_resolved is false and recursors come from the system resolver' do
      let(:properties) do
        {
          'configure_systemd_resolved' => false,
          'systemd_resolved' => {'routing_domains' => ['bosh']},
          'alias_suffixes' => {'bbs.service.cf.internal' => 'service.cf.internal'},
        }
      end

      it 'renders' do
        expect(rendered['systemd_resolved_routing_domains']).to eq(['bosh'])
      end
    end
  end

  describe 'dns/alias_suffixes.json' do
    let(:template) { job.template('dns/alias_suffixes.json') }
    let(:properties) { {} }
    let(:rendered) { JSON.parse(template.render(properties)) }

    it 'defaults to an empty object' do
      expect(rendered).to eq({})
    end

    context 'configured' do
      let(:properties) { {'alias_suffixes' => {'bbs.service.cf.internal' => 'service.cf.internal', '_.under.internal' => 'internal'}} }

      it 'writes the alias suffixes' do
        expect(rendered).to eq({'bbs.service.cf.internal' => 'service.cf.internal', '_.under.internal' => 'internal'})
      end
    end
  end

  describe 'bin/is-system-resolver' do
    let(:template) { job.template('bin/is-system-resolver') }

    context 'on Jammy (override_nameserver: true)' do
      let(:rendered) { template.render({'override_nameserver' => true, 'configure_systemd_resolved' => false}) }

      it 'exits 0 so callers know to wait for bosh-dns' do
        expect(rendered).to include('exit 0')
      end
    end

    context 'on Noble (configure_systemd_resolved: true, override_nameserver: false)' do
      let(:rendered) { template.render({'override_nameserver' => false, 'configure_systemd_resolved' => true}) }

      it 'exits 0 so callers know to wait for bosh-dns to integrate with systemd-resolved' do
        expect(rendered).to include('exit 0')
      end
    end

    context 'when bosh-dns is installed but not acting as any kind of system resolver' do
      let(:rendered) { template.render({'override_nameserver' => false, 'configure_systemd_resolved' => false}) }

      it 'exits 1 so callers skip the wait' do
        expect(rendered).to include('exit 1')
      end
    end
  end

  describe 'bin/wait' do
    let(:template) { job.template('bin/wait') }

    context 'on Jammy (override_nameserver: true)' do
      let(:rendered) { template.render({'override_nameserver' => true, 'configure_systemd_resolved' => false}) }

      it 'checks bosh-dns directly and also via the system resolver' do
        expect(rendered.scan('bosh-dns-wait').length).to eq(2)
      end
    end

    context 'on Noble (configure_systemd_resolved: true, override_nameserver: false)' do
      let(:rendered) { template.render({'override_nameserver' => false, 'configure_systemd_resolved' => true}) }

      it 'checks bosh-dns directly and also via the system resolver' do
        expect(rendered.scan('bosh-dns-wait').length).to eq(2)
      end
    end

    context 'when bosh-dns is not the system resolver' do
      let(:rendered) { template.render({'override_nameserver' => false, 'configure_systemd_resolved' => false}) }

      it 'only checks bosh-dns directly' do
        expect(rendered.scan('bosh-dns-wait').length).to eq(1)
      end
    end
  end

  describe 'config/bpm.yml' do
    let(:template) { job.template('config/bpm.yml') }

    def render(properties = {})
      YAML.safe_load(template.render(properties))
    end

    def process_named(bpm, name)
      bpm.fetch('processes').find { |p| p['name'] == name }
    end

    context 'with default properties (health disabled)' do
      let(:bpm) { render({}) }

      it 'defines the bosh-dns process running the bosh-dns binary' do
        dns = process_named(bpm, 'bosh-dns')
        expect(dns).not_to be_nil
        expect(dns['executable']).to eq('/var/vcap/packages/bosh-dns/bin/bosh-dns')
      end

      it 'passes the absolute config path so the daemon does not depend on cwd' do
        dns = process_named(bpm, 'bosh-dns')
        expect(dns['args']).to eq(['--config', '/var/vcap/jobs/bosh-dns/config/config.json'])
      end

      it 'grants NET_BIND_SERVICE so it can bind :53 as vcap' do
        dns = process_named(bpm, 'bosh-dns')
        expect(dns['capabilities']).to include('NET_BIND_SERVICE')
      end

      it 'preserves the bosh-dns open_files limit of 65536' do
        dns = process_named(bpm, 'bosh-dns')
        expect(dns['limits']['open_files']).to eq(65536)
      end

      it 'does not define the health process when health is disabled' do
        expect(process_named(bpm, 'bosh-dns-health')).to be_nil
        expect(bpm.fetch('processes').length).to eq(1)
      end
    end

    context 'when health.enabled is true' do
      let(:bpm) { render({'health' => {'enabled' => true}}) }

      it 'defines the bosh-dns-health process with the absolute config arg' do
        health = process_named(bpm, 'bosh-dns-health')
        expect(health).not_to be_nil
        expect(health['executable']).to eq('/var/vcap/packages/bosh-dns/bin/bosh-dns-health')
        expect(health['args']).to eq(['/var/vcap/jobs/bosh-dns/config/health_server_config.json'])
      end

      it 'preserves the health server open_files limit of 4096' do
        health = process_named(bpm, 'bosh-dns-health')
        expect(health['limits']['open_files']).to eq(4096)
      end
    end
  end

  describe 'bin/pre-start' do
    let(:template) { job.template('bin/pre-start') }

    context 'when use_bpm is false (default)' do
      it 'starts bosh-dns via the ctl script so DNS is available during other jobs pre-starts' do
        rendered = template.render({})
        expect(rendered).to include('/var/vcap/jobs/bosh-dns/bin/bosh_dns_ctl start')
        expect(rendered).not_to include('bpm start bosh-dns')
      end

      context 'when health.enabled is true' do
        it 'starts the health process via ctl before bosh-dns' do
          rendered = template.render({'health' => {'enabled' => true}})
          expect(rendered).to match(%r{bosh_dns_health_ctl start.*bosh_dns_ctl start}m)
          expect(rendered).not_to include('bpm start')
        end
      end

      context 'when health.enabled is false (default)' do
        it 'does not start the health process' do
          rendered = template.render({})
          expect(rendered).not_to include('bosh-dns-health')
          expect(rendered).not_to include('bosh_dns_health_ctl')
        end
      end
    end

    context 'when use_bpm is true' do
      it 'starts bosh-dns via bpm so DNS is available during other jobs pre-starts' do
        rendered = template.render({'use_bpm' => true})
        expect(rendered).to include('/var/vcap/jobs/bpm/bin/bpm start bosh-dns')
        expect(rendered).not_to include('bosh_dns_ctl start')
      end

      context 'when health.enabled is true' do
        it 'starts the health process via bpm before bosh-dns' do
          rendered = template.render({'use_bpm' => true, 'health' => {'enabled' => true}})
          expect(rendered).to match(%r{bpm start bosh-dns -p bosh-dns-health.*bpm start bosh-dns$}m)
        end
      end

      context 'when health.enabled is false (default)' do
        it 'does not start the health process' do
          rendered = template.render({'use_bpm' => true})
          expect(rendered).not_to include('bosh-dns-health')
        end
      end
    end

    context 'on Jammy (configure_systemd_resolved: false)' do
      let(:rendered) { template.render({'override_nameserver' => true, 'configure_systemd_resolved' => false}) }

      it 'invokes the loopback alias setup and not the systemd-resolved path' do
        expect(rendered).to match(/^create_network_alias$/)
        expect(rendered).not_to include('bosh-dns-systemd-resolved-updater')
      end
    end

    context 'on Noble (configure_systemd_resolved: true)' do
      let(:rendered) { template.render({'override_nameserver' => false, 'configure_systemd_resolved' => true}) }

      it 'invokes the dummy interface setup and runs the systemd-resolved updater' do
        expect(rendered).to match(/^create_network_interface$/)
        expect(rendered).to include('bosh-dns-systemd-resolved-updater')
        expect(rendered).to include('resolvectl default-route bosh-dns no')
      end
    end
  end

  describe 'monit' do
    context 'when use_bpm is false (default)' do
      let(:rendered) { render_monit({}) }

      it 'uses the ctl script pidfile and start/stop programs' do
        expect(rendered).to include('with pidfile /var/vcap/sys/run/bosh-dns/bosh-dns.pid')
        expect(rendered).to include('start program "/var/vcap/jobs/bosh-dns/bin/bosh_dns_ctl start"')
        expect(rendered).to include('stop program "/var/vcap/jobs/bosh-dns/bin/bosh_dns_ctl stop"')
        expect(rendered).not_to include('/var/vcap/jobs/bpm/bin/bpm')
      end

      context 'when health.enabled is true' do
        let(:rendered) { render_monit({'health' => {'enabled' => true}}) }

        it 'uses the health ctl script pidfile and start/stop programs' do
          expect(rendered).to include('with pidfile /var/vcap/sys/run/bosh-dns/bosh_dns_health.pid')
          expect(rendered).to include('start program "/var/vcap/jobs/bosh-dns/bin/bosh_dns_health_ctl start"')
          expect(rendered).to include('stop program "/var/vcap/jobs/bosh-dns/bin/bosh_dns_health_ctl stop"')
        end
      end
    end

    context 'when use_bpm is true' do
      let(:rendered) { render_monit({'use_bpm' => true}) }

      it 'uses the bpm pidfile and start/stop programs' do
        expect(rendered).to include('with pidfile /var/vcap/sys/run/bpm/bosh-dns/bosh-dns.pid')
        expect(rendered).to include('start program "/var/vcap/jobs/bpm/bin/bpm start bosh-dns"')
        expect(rendered).to include('stop program "/var/vcap/jobs/bpm/bin/bpm stop bosh-dns"')
        expect(rendered).not_to include('bosh_dns_ctl')
      end

      context 'when health.enabled is true' do
        let(:rendered) { render_monit({'use_bpm' => true, 'health' => {'enabled' => true}}) }

        it 'uses the bpm health pidfile and start/stop programs' do
          expect(rendered).to include('with pidfile /var/vcap/sys/run/bpm/bosh-dns/bosh-dns-health.pid')
          expect(rendered).to include('start program "/var/vcap/jobs/bpm/bin/bpm start bosh-dns -p bosh-dns-health"')
          expect(rendered).to include('stop program "/var/vcap/jobs/bpm/bin/bpm stop bosh-dns -p bosh-dns-health"')
        end
      end
    end
  end

end
