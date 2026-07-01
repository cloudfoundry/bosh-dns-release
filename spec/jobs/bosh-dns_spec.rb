require_relative 'shared_examples'

describe 'bosh-dns' do
  let(:release) { Bosh::Template::Test::ReleaseDir.new(File.join(File.dirname(__FILE__), '../..')) }
  let(:job) { release.job('bosh-dns') }

  it_behaves_like 'common config.json'

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

    it 'starts bosh-dns via bpm so DNS is available during other jobs pre-starts' do
      rendered = template.render({})
      expect(rendered).to include('/var/vcap/jobs/bpm/bin/bpm start bosh-dns')
    end

    context 'when health.enabled is true' do
      it 'starts the health process before bosh-dns' do
        rendered = template.render({'health' => {'enabled' => true}})
        expect(rendered).to match(%r{bpm start bosh-dns -p bosh-dns-health.*bpm start bosh-dns$}m)
      end
    end

    context 'when health.enabled is false (default)' do
      it 'does not start the health process' do
        rendered = template.render({})
        expect(rendered).not_to include('bosh-dns-health')
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
      end
    end
  end

end
