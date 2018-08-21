package main

import "testing"

func TestCloudinaryTransform(t *testing.T) {
	type args struct {
		url        string
		transforms string
	}
	tests := []struct {
		name               string
		args               args
		wantTransformedURL string
		wantErr            bool
	}{
		{
			name: "Basic",
			args: args{
				url:        "https://res.cloudinary.com/unee-t-dev/image/upload/attachments/zd7reouq5i85pmsqrh70.jpg",
				transforms: "c_fill,g_auto,h_500,w_500",
			},
			wantTransformedURL: "https://res.cloudinary.com/unee-t-dev/c_fill,g_auto,h_500,w_500/attachments/zd7reouq5i85pmsqrh70.jpg",
			wantErr:            false,
		},
		{
			name: "Not a URL",
			args: args{
				url: "Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
			},
			wantErr: true,
		},
		{
			name: "Already transformed",
			args: args{
				url:        "http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
				transforms: "c_fill,g_auto,h_500,w_500",
			},
			wantTransformedURL: "http://res.cloudinary.com/unee-t-staging/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
			wantErr:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTransformedURL, err := CloudinaryTransform(tt.args.url, tt.args.transforms)
			if (err != nil) != tt.wantErr {
				t.Errorf("CloudinaryTransform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotTransformedURL != tt.wantTransformedURL {
				t.Errorf("CloudinaryTransform() = %v, want %v", gotTransformedURL, tt.wantTransformedURL)
			}
		})
	}
}
