package config

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"
)

func Test_loadViperConfig(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "sanity",
			args:    struct{ filePath string }{filePath: "/home/deepak/Projects/notiboy-backend/etc/config.yaml"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := loadViperConfig(tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("loadViperConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			fmt.Println(viper.Sub("algorand").AllSettings())
		})
	}
}
