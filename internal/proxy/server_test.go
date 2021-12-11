package proxy

import (
	"testing"
)

func TestServer_Start(t *testing.T) {
	type fields struct {
		Config *Config
		cache  Cache
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				Config: tt.fields.Config,
				cache:  tt.fields.cache,
			}
			s.Start()
		})
	}
}
