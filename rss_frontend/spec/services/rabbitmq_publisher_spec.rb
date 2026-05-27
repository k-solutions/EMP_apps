require 'rails_helper'

RSpec.describe RabbitmqPublisher do
  let(:routing_key) { "rss_commands_worker" }
  let(:payload) { { job_id: "test_job", urls: ["https://example.com/rss"] }.to_json }

  before do
    @mock_conn = instance_double(Bunny::Session)
    @mock_channel = instance_double(Bunny::Channel)
    @mock_exchange = instance_double(Bunny::Exchange)

    allow(Bunny).to receive(:new).and_return(@mock_conn)
    allow(@mock_conn).to receive(:start).and_return(true)
    allow(@mock_conn).to receive(:create_channel).and_return(@mock_channel)
    allow(@mock_channel).to receive(:direct).with("rss_commands", durable: true).and_return(@mock_exchange)
    allow(@mock_conn).to receive(:close).and_return(true)
  end

  describe "#publish" do
    it "publishes payload with correct routing key to the direct exchange with persistence" do
      expect(@mock_exchange).to receive(:publish).with(payload, routing_key: routing_key, persistent: true)

      publisher = RabbitmqPublisher.new
      publisher.publish(routing_key: routing_key, payload: payload)
      publisher.close
    end
  end
end
