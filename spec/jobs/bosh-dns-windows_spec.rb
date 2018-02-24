require_relative 'shared_examples'

describe 'bosh-dns-windows' do
  let(:release) { Bosh::Template::Test::ReleaseDir.new(File.join(File.dirname(__FILE__), '../..')) }
  let(:job) { release.job('bosh-dns-windows') }

  it_behaves_like 'common config.json'
end
