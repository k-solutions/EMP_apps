//go:build integration

package worker_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// Helper structure to extract schemas from AsyncAPI components section
type AsyncAPIContract struct {
	Components struct {
		Messages struct {
			CommandMessage struct {
				Payload interface{} `yaml:"payload"`
			} `yaml:"CommandMessage"`
			ResultMessage struct {
				Payload interface{} `yaml:"payload"`
			} `yaml:"ResultMessage"`
		} `yaml:"messages"`
	} `yaml:"components"`
}

func LoadSchemaFromAsyncAPI(t *testing.T, messageType string) *gojsonschema.Schema {
	yamlBytes, err := os.ReadFile("../docs/asyncapi.yaml")
	if err != nil {
		t.Fatalf("Failed to read AsyncAPI contract definition: %v", err)
	}

	var contract AsyncAPIContract
	if err := yaml.Unmarshal(yamlBytes, &contract); err != nil {
		t.Fatalf("Failed to parse AsyncAPI spec yaml: %v", err)
	}

	var rawPayload interface{}
	if messageType == "CommandMessage" {
		rawPayload = contract.Components.Messages.CommandMessage.Payload
	} else {
		rawPayload = contract.Components.Messages.ResultMessage.Payload
	}

	jsonBytes, err := json.Marshal(rawPayload)
	if err != nil {
		t.Fatalf("Failed to convert YAML schema map to JSON: %v", err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(jsonBytes)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		t.Fatalf("Failed compilation compile JSON Schema from contract: %v", err)
	}
	return schema
}

func TestVerifyCommandPayloadContractConformance(t *testing.T) {
	schema := LoadSchemaFromAsyncAPI(t, "CommandMessage")

	// Target payload received over mock or test network interface exchange broker
	samplePayload := `{"job_id": "01J3KPENDING0000000000000", "urls": ["https://feeds.bbci.co.uk/news/rss.xml"]}`

	documentLoader := gojsonschema.NewStringLoader(samplePayload)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		t.Fatalf("Validation error occurred during execution: %v", err)
	}

	if !result.Valid() {
		t.Errorf("Payload violates AsyncAPI explicit command structure contract rules:")
		for _, desc := range result.Errors() {
			t.Errorf("- %s", desc)
		}
	}
}

func TestVerifyResultPayloadContractConformance(t *testing.T) {
	schema := LoadSchemaFromAsyncAPI(t, "ResultMessage")

	// Target payload representing outbound parsing outcome published by worker
	samplePayload := `{
		"job_id": "01J3KPENDING0000000000000",
		"status": "done",
		"items": [
			{
				"title": "Example Feed Item",
				"source": "BBC News",
				"source_url": "https://feeds.bbci.co.uk/news/rss.xml",
				"link": "https://www.bbc.co.uk/news/12345",
				"publish_date": "2026-05-25",
				"description": "Content description text."
			}
		],
		"errors": []
	}`

	documentLoader := gojsonschema.NewStringLoader(samplePayload)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		t.Fatalf("Validation error occurred during execution: %v", err)
	}

	if !result.Valid() {
		t.Errorf("Payload violates AsyncAPI explicit result structure contract rules:")
		for _, desc := range result.Errors() {
			t.Errorf("- %s", desc)
		}
	}
}
