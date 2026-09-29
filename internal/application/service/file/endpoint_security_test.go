package file

import "testing"

func TestS3ClientRejectsUnsafeEndpointAtConstruction(t *testing.T) {
	const endpoint = "http://169.254.169.254/latest/meta-data"
	_, err := newS3Client(S3Options{
		Endpoint: endpoint, AccessKey: "ak", SecretKey: "sk", BucketName: "bucket", Region: "region",
		AddressingStyle: "path",
	})
	if err == nil {
		t.Fatal("expected unsafe endpoint to be rejected")
	}
}
