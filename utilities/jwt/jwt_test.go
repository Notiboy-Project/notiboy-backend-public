package jwt

import (
	"fmt"
	"testing"

	"github.com/spf13/cast"

	"notiboy/config"
)

func TestGenerateJWT(t *testing.T) {
	config.LoadConfig()
	config.GetConfig().LoginTokenExpiry = "999h"
	type args struct {
		address    string
		chain      string
		membership string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "sanity - xrpl",
			args: args{
				"rLagCDBn2P2u1DJzrBu4o7cwjMjY9JVshx",
				"xrpl",
				"free",
			},
			wantErr: false,
		},
		{
			name: "sanity - algorand",
			args: args{
				address:    "EMAVMBG5P4AHJBNDSJSFH2USQSWIE6QQOVQLAPGXI2HQ3OJ7ILH6CDMOVU",
				chain:      "algorand",
				membership: "free",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, _, err := GenerateJWT(tt.args.address, tt.args.chain, "", "", cast.ToDuration("9999h"))
				if (err != nil) != tt.wantErr {
					t.Errorf("GenerateJWT() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				fmt.Println(got)
			},
		)
	}
}
