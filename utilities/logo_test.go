package utilities

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"testing"
)

func loadImage(kind string) (string, error) {
	var (
		err   error
		bytes []byte
	)

	switch kind {
	case "png":
		bytes, err = os.ReadFile("../testdata/logo.png")
	case "jpg":
		bytes, err = os.ReadFile("../testdata/logo.jpg")
	}
	if err != nil {
		return "", err
	}

	mimeType := http.DetectContentType(bytes)
	fmt.Println("Mime Type:", mimeType)

	base64Encoding := base64.StdEncoding.EncodeToString(bytes)

	return base64Encoding, nil
}

func Test_validateImage(t *testing.T) {
	type args struct {
		bytes          string
		x              int
		y              int
		size           int
		supportedTypes []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "png - sanity",
			args: args{
				bytes: func() string {
					b, err := loadImage("png")
					if err != nil {
						t.Error(err)
					}
					return b
				}(),
				x:              50,
				y:              50,
				size:           256,
				supportedTypes: []string{"png", "jpg", "jpeg"},
			},
			wantErr: false,
		},
		{
			name: "png - x,y pixel breach",
			args: args{
				bytes: func() string {
					b, err := loadImage("png")
					if err != nil {
						t.Error(err)
					}
					return b
				}(),
				x:              25,
				y:              25,
				size:           256,
				supportedTypes: []string{"png", "jpg", "jpeg"},
			},
			wantErr: true,
		},
		{
			name: "png - x,y size breach",
			args: args{
				bytes: func() string {
					b, err := loadImage("png")
					if err != nil {
						t.Error(err)
					}
					return b
				}(),
				x:              50,
				y:              50,
				size:           3,
				supportedTypes: []string{"png", "jpg", "jpeg"},
			},
			wantErr: true,
		},
		{
			name: "png - x,y unsupported type",
			args: args{
				bytes: func() string {
					b, err := loadImage("png")
					if err != nil {
						t.Error(err)
					}
					return b
				}(),
				x:              50,
				y:              50,
				size:           3,
				supportedTypes: []string{"png", "jpg", "jpeg"},
			},
			wantErr: true,
		},
		{
			name: "jpg - sanity",
			args: args{
				bytes: func() string {
					b, err := loadImage("jpg")
					if err != nil {
						t.Error(err)
					}
					return b
				}(),
				x:              2800,
				y:              2800,
				size:           870,
				supportedTypes: []string{"jpg", "jpeg"},
			},
			wantErr: false,
		},
		{
			name: "jpg - expected jpeg",
			args: args{
				bytes: func() string {
					b, err := loadImage("jpg")
					if err != nil {
						t.Error(err)
					}
					return b
				}(),
				x:              2800,
				y:              2800,
				size:           870,
				supportedTypes: []string{"jpeg"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImage(tt.args.bytes, tt.args.x, tt.args.y, tt.args.size, tt.args.supportedTypes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
