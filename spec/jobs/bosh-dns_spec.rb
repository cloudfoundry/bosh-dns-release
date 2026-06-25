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

  describe 'bin/bosh_dns_ctl' do
    let(:template) { job.template('bin/bosh_dns_ctl') }
    let(:rendered) { template.render({}) }

    it 'does not reference chpst' do
      expect(rendered).not_to include('chpst')
    end

    it 'uses setpriv to drop privileges to vcap' do
      expect(rendered).to include('setpriv --reuid=vcap --regid=vcap --clear-groups --')
    end

    it 'does not use --no-new-privs because bosh-dns relies on cap_net_bind_service file capability for port 53' do
      expect(rendered).not_to include('--no-new-privs')
    end
  end

  describe 'bin/bosh_dns_health_ctl' do
    let(:template) { job.template('bin/bosh_dns_health_ctl') }
    let(:rendered) { template.render({}) }

    it 'does not reference chpst' do
      expect(rendered).not_to include('chpst')
    end

    it 'uses setpriv with --no-new-privs to drop privileges to vcap' do
      expect(rendered).to include('setpriv --reuid=vcap --regid=vcap --clear-groups --no-new-privs --')
    end
  end

end
