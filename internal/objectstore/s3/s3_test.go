package s3

import "testing"

func TestYandexProviderDefaults(t *testing.T) {
	store, err := New(Config{
		Provider:        "yandex",
		Bucket:          "iaon-test-bucket",
		Region:          "ru-central1-a",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.cfg.Endpoint != "https://storage.yandexcloud.net" {
		t.Fatalf("endpoint = %q", store.cfg.Endpoint)
	}
	if store.cfg.Region != "ru-central1" {
		t.Fatalf("region = %q", store.cfg.Region)
	}
	if !store.cfg.ForcePathStyle {
		t.Fatal("Yandex provider should default to path-style URLs")
	}
	u, err := store.objectURL("prefix/object.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := u.String(), "https://storage.yandexcloud.net/iaon-test-bucket/prefix/object.txt"; got != want {
		t.Fatalf("object URL = %q, want %q", got, want)
	}
}

func TestAWSProviderDefaultURL(t *testing.T) {
	store, err := New(Config{
		Provider:        "aws",
		Bucket:          "demo-bucket",
		Region:          "eu-central-1",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.objectURL("prefix/object.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := u.String(), "https://demo-bucket.s3.eu-central-1.amazonaws.com/prefix/object.txt"; got != want {
		t.Fatalf("object URL = %q, want %q", got, want)
	}
}

func TestMinIOProviderDefaults(t *testing.T) {
	store, err := New(Config{
		Provider:        "minio",
		Bucket:          "s3s5-test",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.cfg.Endpoint != "http://127.0.0.1:9000" {
		t.Fatalf("endpoint = %q", store.cfg.Endpoint)
	}
	if !store.cfg.ForcePathStyle {
		t.Fatal("MinIO provider should default to path-style URLs")
	}
	u, err := store.objectURL("prefix/object.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := u.String(), "http://127.0.0.1:9000/s3s5-test/prefix/object.txt"; got != want {
		t.Fatalf("object URL = %q, want %q", got, want)
	}
}
