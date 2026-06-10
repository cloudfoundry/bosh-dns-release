require_relative 'shared_examples'

describe 'bosh-dns' do
  let(:release) { Bosh::Template::Test::ReleaseDir.new(File.join(File.dirname(__FILE__), '../..')) }
  let(:job) { release.job('bosh-dns') }

  it_behaves_like 'common config.json'

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
