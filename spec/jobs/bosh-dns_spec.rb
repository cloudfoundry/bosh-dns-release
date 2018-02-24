require_relative 'shared_examples'

describe 'bosh-dns' do
  let(:release) { Bosh::Template::Test::ReleaseDir.new(File.join(File.dirname(__FILE__), '../..')) }
  let(:job) { release.job('bosh-dns') }

  it_behaves_like 'common config.json'

end
