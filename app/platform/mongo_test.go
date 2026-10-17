package platform

import "testing"

func TestMongoDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, uri, configured, want string
	}{
		{name: "uri path", uri: "mongodb://localhost:27017/shop", want: "shop"},
		{name: "configured wins", uri: "mongodb://localhost:27017/shop", configured: "audit", want: "audit"},
		{
			name: "railway plugin url without path",
			uri:  "mongodb://mongo:secret@mongodb.railway.internal:27017",
			want: DefaultMongoDatabase,
		},
		{
			name: "path with authSource",
			uri:  "mongodb://u:p@host:27017/marketplace?authSource=admin",
			want: "marketplace",
		},
		{name: "no path plus configured", uri: "mongodb://host:27017", configured: "mkt", want: "mkt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := MongoDatabase(tt.uri, tt.configured)
			if err != nil {
				t.Fatalf("MongoDatabase(%q, %q) error: %v", tt.uri, tt.configured, err)
			}

			if got != tt.want {
				t.Errorf("MongoDatabase(%q, %q) = %q, want %q", tt.uri, tt.configured, got, tt.want)
			}
		})
	}

	if _, err := MongoDatabase("http://not-mongo", ""); err == nil {
		t.Error("MongoDatabase accepted a non-mongodb URI")
	}
}
