require 'json-schema'
require 'yaml'

RSpec::Matchers.define :comply_with_asyncapi_spec do |message_type|
  match do |json_payload|
    asyncapi_path = Rails.root.join('config', 'asyncapi.yaml')
    spec = YAML.load_file(asyncapi_path)
    
    # Isolate payload schema definition target parameters from components list
    raw_schema = spec.dig('components', 'messages', message_type.to_s, 'payload')
    
    raise "Target message blueprint type '#{message_type}' unmapped inside AsyncAPI spec" if raw_schema.nil?

    # Target data parser initialization handles structural evaluation checks
    payload_data = json_payload.is_a?(String) ? JSON.parse(json_payload) : json_payload
    
    @errors = JSON::Validator.fully_validate(raw_schema, payload_data)
    @errors.empty?
  end

  failure_message do |json_payload|
    "Expected target execution packet payload to correspond directly to AsyncAPI Contract criteria [#{message_type}]. Detected errors:\n#{@errors.join("\n")}"
  end
end
