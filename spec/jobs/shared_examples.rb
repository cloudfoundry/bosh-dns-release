require 'rspec'
require 'json'
require 'bosh/template/test'
require 'yaml'

shared_examples_for 'common config.json' do
  describe 'config/config.json' do
    let(:template) { job.template('config/config.json') }
    let(:properties) { {} }
    let(:rendered) { JSON.parse(template.render(properties)) }

    context 'excluded_recursors' do
      it 'defaults to empty' do
        expect(rendered['excluded_recursors']).to eq([])
      end

      context 'configured' do
        let(:properties) { {'excluded_recursors' => ['192.0.2.1']} }

        it 'writes excluded_recursors' do
          expect(rendered['excluded_recursors']).to eq(['192.0.2.1'])
        end
      end
    end
  end
end
