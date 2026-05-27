require 'rails_helper'

RSpec.describe ProcessFeedResultJob, type: :job do
  fixtures :users, :feed_requests

  let(:worker) { ProcessFeedResultJob.new }
  let(:user) { users(:alice) }
  let(:feed_request) { feed_requests(:alice_pending) }
  let(:payload) do
    {
      job_id: feed_request.job_id,
      status: "done",
      items: [
        {
          "title" => "Parsed News",
          "link" => "https://example.com/parsed-1",
          "source" => "BBC News",
          "source_url" => "https://feeds.bbci.co.uk/news/rss.xml",
          "publish_date" => "2026-05-24",
          "description" => "Something parsed"
        }
      ],
      errors: []
    }.to_json
  end

  describe "Sneakers Configuration" do
    it "verifies queue and exchange configurations are direct and durable" do
      opts = ProcessFeedResultJob.queue_opts
      expect(ProcessFeedResultJob.queue_name).to eq("rss_results_rails")
      expect(opts[:exchange]).to eq("rss_results")
      expect(opts[:exchange_type]).to eq(:direct)
      expect(opts[:durable]).to be(true)
    end
  end

  describe "Contract Conformance" do
    it "verifies that the inbound message complies with the ResultMessage contract schema" do
      expect(payload).to comply_with_asyncapi_spec(:ResultMessage)
    end
  end

  describe "#work" do
    it "processes the result, saves feed items, updates request status and sends ActionCable broadcast" do
      expect(ActionCable.server).to receive(:broadcast).with(
        "feed_#{user.id}",
        hash_including(feed_request_id: feed_request.id, status: "done")
      )

      expect {
        res = worker.work(payload)
        expect(res).to eq(:ack)
      }.to change(FeedItem, :count).by(1)

      feed_request.reload
      expect(feed_request.status).to eq("done")
      expect(FeedItem.last.title).to eq("Parsed News")
    end

    it "acks and ignores if job_id is unknown" do
      unknown_payload = { job_id: "unknown_job_123", status: "done", items: [], errors: [] }.to_json
      res = worker.work(unknown_payload)
      expect(res).to eq(:ack)
    end

    it "acks and ignores if request status is already done (idempotency)" do
      feed_request.update!(status: "done")

      expect {
        res = worker.work(payload)
        expect(res).to eq(:ack)
      }.not_to change(FeedItem, :count)
    end

    it "rejects the message if an unexpected error occurs" do
      target_payload = payload
      allow(FeedRequest).to receive(:find_by).and_raise(StandardError.new("DB connection error"))
      res = worker.work(target_payload)
      expect(res).to eq(:reject)
    end
  end
end
