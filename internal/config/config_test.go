package config

import "testing"

func TestYandexProviderFinalizesEndpointAndRegion(t *testing.T) {
	c := Common{
		Provider:         "yc",
		Bucket:           "bucket",
		Prefix:           "prefix",
		Region:           "ru-central1-a",
		PSK:              "0123456789abcdef",
		ChunkSize:        1,
		PollMin:          1,
		PollMax:          1,
		WindowChunks:     1,
		InsecureNoCrypto: true,
	}
	if err := c.Finalize(true); err != nil {
		t.Fatal(err)
	}
	if c.Provider != "yandex" {
		t.Fatalf("provider = %q", c.Provider)
	}
	if c.Endpoint != "https://storage.yandexcloud.net" {
		t.Fatalf("endpoint = %q", c.Endpoint)
	}
	if c.Region != "ru-central1" {
		t.Fatalf("region = %q", c.Region)
	}
	if !c.ForcePathStyle {
		t.Fatal("expected path-style URLs for Yandex")
	}
}
