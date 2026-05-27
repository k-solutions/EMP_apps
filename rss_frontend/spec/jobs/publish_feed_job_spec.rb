require 'rails_helper'

RSpec.describe PublishFeedJob, type: :job do
  fixtures :users

  let(:feed_request) do
    FeedRequest.create!(
      user: users(:alice),
      job_id: "01J3KPENDING0000000000000",
      urls: ["https://feeds.bbci.co.uk/news/rss.xml"],
      status: "pending"
    )
  end

  describe "#perform" do
    context "when RabbitMQ is healthy" do
      it "verifies that asynchronous message generation matches the command contract exactly" do
        publisher_mock = instance_double(RabbitmqPublisher)
        allow(RabbitmqPublisher).to receive(:new).and_return(publisher_mock)

        captured_payload = nil
        expect(publisher_mock).to receive(:publish) do |args|
          captured_payload = args[:payload]
        end
        expect(publisher_mock).to receive(:close).and_return(true)

        PublishFeedJob.new.perform(feed_request)

        # Programmatic contract assertion against config/asyncapi.yaml
        expect(captured_payload).to comply_with_asyncapi_spec(:CommandMessage)
        
        feed_request.reload
        expect(feed_request.status).to eq("processing")
      end
    end

    context "when RabbitMQ fails (Fallback Mode)" do
      let(:go_parsed_body) do
        {
          job_id: "NEW_JOB_ID_123",
          status: "done",
          items: [
            {
              title: "Direct Fallback Item",
              source: "Go service",
              source_url: "https://feeds.bbci.co.uk/news/rss.xml",
              link: "https://example.com/fallback-1",
              publish_date: "2026-05-25",
              description: "Directly fetched"
            }
          ],
          errors: []
        }.to_json
      end

      before do
        # Force publisher initialization to raise an error
        allow(RabbitmqPublisher).to receive(:new).and_raise(StandardError.new("RabbitMQ connection refused"))
        
        # Stub JWT generation
        allow(JwtService).to receive(:generate_token).and_return("mocked_jwt_token")
      end

      it "synchronously requests Go RSS service and inserts items" do
        # Stub HTTP request
        stub_request(:post, "http://localhost:8080/parse")
          .with(
            body: { urls: feed_request.urls }.to_json,
            headers: {
              'Authorization' => 'Bearer mocked_jwt_token',
              'Content-Type' => 'application/json'
            }
          )
          .to_return(status: 200, body: go_parsed_body, headers: {})

        expect(ActionCable.server).to receive(:broadcast).with(
          "feed_#{feed_request.user_id}",
          hash_including(status: "done", feed_request_id: feed_request.id)
        )

        expect {
          PublishFeedJob.new.perform(feed_request)
        }.to change(FeedItem, :count).by(1)

        feed_request.reload
        expect(feed_request.job_id).to eq("NEW_JOB_ID_123")
        expect(feed_request.status).to eq("done")
      end

      it "marks the request as failed when Go service fails" do
        stub_request(:post, "http://localhost:8080/parse")
          .to_return(status: 500, body: "", headers: {})

        expect(ActionCable.server).to receive(:broadcast).with(
          "feed_#{feed_request.user_id}",
          hash_including(status: "failed", feed_request_id: feed_request.id)
        )

        PublishFeedJob.new.perform(feed_request)

        feed_request.reload
        expect(feed_request.status).to eq("failed")
      end
    end
  end
end
